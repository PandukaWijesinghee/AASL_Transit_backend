-- ============================================================================
-- Migration: Bus Owner Onboarding and Permit Management
-- Version: 001
-- Description: Updates to support mandatory onboarding flow and route permits
-- Date: 2025-01-XX
-- ============================================================================

-- ============================================================================
-- 1. UPDATE BUS_OWNERS TABLE
-- ============================================================================

-- Add profile_completed column to track onboarding completion
ALTER TABLE bus_owners
ADD COLUMN IF NOT EXISTS profile_completed BOOLEAN DEFAULT false;

-- Add identity_or_incorporation_no column (replaces license_number)
ALTER TABLE bus_owners
ADD COLUMN IF NOT EXISTS identity_or_incorporation_no VARCHAR(100);

-- In production, migrate existing license_number data to identity_or_incorporation_no first
-- UPDATE bus_owners SET identity_or_incorporation_no = license_number WHERE license_number IS NOT NULL;

-- Then drop license_number column
-- ALTER TABLE bus_owners DROP COLUMN IF EXISTS license_number;

-- Add comments
COMMENT ON COLUMN bus_owners.identity_or_incorporation_no IS 'Identity Card Number (NIC) or Company Incorporation Number';
COMMENT ON COLUMN bus_owners.profile_completed IS 'Indicates if bus owner has completed mandatory onboarding (company profile + at least one permit)';

-- ============================================================================
-- 2. CREATE ROUTE_PERMITS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS route_permits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Ownership
    bus_owner_id UUID NOT NULL REFERENCES bus_owners(id) ON DELETE CASCADE,

    -- Permit identification
    permit_number VARCHAR(100) NOT NULL,

    -- Bus registration (1:1 relationship with bus)
    bus_registration_number VARCHAR(50) NOT NULL,

    -- Route information
    master_route_id UUID, -- Optional reference to master_routes if exists
    route_number VARCHAR(50) NOT NULL,
    route_name VARCHAR(255) NOT NULL,
    full_origin_city VARCHAR(100) NOT NULL,
    full_destination_city VARCHAR(100) NOT NULL,
    via TEXT[], -- Array of intermediate stops (e.g., ARRAY['Kadawatha', 'Kegalle'])
    total_distance_km DECIMAL(8,2),
    estimated_duration_minutes INTEGER,

    -- Permit details
    issue_date DATE NOT NULL,
    expiry_date DATE NOT NULL,
    permit_type VARCHAR(50) DEFAULT 'regular',
    approved_fare DECIMAL(10,2) NOT NULL, -- Government approved fare

    -- Restrictions
    max_trips_per_day INTEGER,
    allowed_bus_types TEXT[], -- e.g., ARRAY['ac', 'deluxe', 'semi_sleeper']
    restrictions TEXT,

    -- Status and verification
    status verification_status DEFAULT 'pending',
    verified_at TIMESTAMP WITH TIME ZONE,
    permit_document_url TEXT, -- URL to uploaded permit document

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT route_permits_bus_owner_id_not_null CHECK (bus_owner_id IS NOT NULL),
    CONSTRAINT route_permits_approved_fare_positive CHECK (approved_fare > 0),
    CONSTRAINT route_permits_expiry_after_issue CHECK (expiry_date > issue_date),
    CONSTRAINT route_permits_permit_bus_unique UNIQUE (bus_owner_id, permit_number)
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_route_permits_bus_owner_id ON route_permits(bus_owner_id);
CREATE INDEX IF NOT EXISTS idx_route_permits_bus_registration ON route_permits(bus_registration_number);
CREATE INDEX IF NOT EXISTS idx_route_permits_permit_number ON route_permits(permit_number);
CREATE INDEX IF NOT EXISTS idx_route_permits_status ON route_permits(status);
CREATE INDEX IF NOT EXISTS idx_route_permits_expiry ON route_permits(expiry_date);
CREATE INDEX IF NOT EXISTS idx_route_permits_route_number ON route_permits(route_number);

-- Add comments
COMMENT ON TABLE route_permits IS 'Government-issued route permits for bus owners';
COMMENT ON COLUMN route_permits.bus_registration_number IS 'License plate number from the permit (links to buses.license_plate)';
COMMENT ON COLUMN route_permits.via IS 'Array of intermediate stops as specified in the permit';
COMMENT ON COLUMN route_permits.approved_fare IS 'Government approved fare for this route (in LKR)';
COMMENT ON COLUMN route_permits.permit_document_url IS 'URL to uploaded permit document (PDF/image)';

-- ============================================================================
-- 3. CREATE ROUTE_PERMIT_STOPS TABLE (optional - for predefined stops)
-- ============================================================================

CREATE TABLE IF NOT EXISTS route_permit_stops (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_permit_id UUID NOT NULL REFERENCES route_permits(id) ON DELETE CASCADE,

    -- Stop details
    stop_name VARCHAR(255) NOT NULL,
    stop_order INTEGER NOT NULL,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    arrival_time_offset_minutes INTEGER, -- Minutes from route start
    is_major_stop BOOLEAN DEFAULT false,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(route_permit_id, stop_order)
);

CREATE INDEX IF NOT EXISTS idx_route_permit_stops_permit_id ON route_permit_stops(route_permit_id);

COMMENT ON TABLE route_permit_stops IS 'Predefined stops from master route for each permit';

-- ============================================================================
-- 4. UPDATE FUNCTION FOR updated_at TRIGGER
-- ============================================================================

-- Create or replace the updated_at trigger function if it doesn't exist
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add trigger for bus_owners (if not exists)
DROP TRIGGER IF EXISTS update_bus_owners_updated_at ON bus_owners;
CREATE TRIGGER update_bus_owners_updated_at
    BEFORE UPDATE ON bus_owners
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add trigger for route_permits
DROP TRIGGER IF EXISTS update_route_permits_updated_at ON route_permits;
CREATE TRIGGER update_route_permits_updated_at
    BEFORE UPDATE ON route_permits
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 5. HELPER FUNCTIONS
-- ============================================================================

-- Function to check if a permit is valid
CREATE OR REPLACE FUNCTION is_permit_valid(permit_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    permit_status verification_status;
    permit_expiry DATE;
BEGIN
    SELECT status, expiry_date INTO permit_status, permit_expiry
    FROM route_permits
    WHERE id = permit_id;

    RETURN permit_status = 'verified'
        AND permit_expiry >= CURRENT_DATE;
END;
$$ LANGUAGE plpgsql;

-- Function to check if permit is expiring soon (within 30 days)
CREATE OR REPLACE FUNCTION is_permit_expiring_soon(permit_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    permit_expiry DATE;
    days_until_expiry INTEGER;
BEGIN
    SELECT expiry_date INTO permit_expiry
    FROM route_permits
    WHERE id = permit_id;

    days_until_expiry := permit_expiry - CURRENT_DATE;

    RETURN days_until_expiry <= 30 AND days_until_expiry > 0;
END;
$$ LANGUAGE plpgsql;

-- Function to automatically update profile_completed when first permit is added
CREATE OR REPLACE FUNCTION update_bus_owner_profile_completed()
RETURNS TRIGGER AS $$
BEGIN
    -- When a new permit is added, check if this is the owner's first permit
    -- If so, mark profile as completed
    UPDATE bus_owners
    SET profile_completed = TRUE
    WHERE id = NEW.bus_owner_id
    AND profile_completed = FALSE
    AND EXISTS (
        SELECT 1 FROM route_permits
        WHERE bus_owner_id = NEW.bus_owner_id
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update profile_completed
DROP TRIGGER IF EXISTS auto_complete_profile ON route_permits;
CREATE TRIGGER auto_complete_profile
    AFTER INSERT ON route_permits
    FOR EACH ROW
    EXECUTE FUNCTION update_bus_owner_profile_completed();

-- ============================================================================
-- 6. SAMPLE DATA (FOR DEVELOPMENT ONLY - Comment out for production)
-- ============================================================================

-- Insert sample bus owner (assuming user exists)
/*
DO $$
DECLARE
    v_user_id UUID;
    v_bus_owner_id UUID;
BEGIN
    -- Create a test user if doesn't exist
    INSERT INTO users (phone, first_name, last_name, roles, profile_completed, phone_verified, status)
    VALUES ('0771234567', 'Test', 'Bus Owner', ARRAY['bus_owner'], TRUE, TRUE, 'active')
    ON CONFLICT (phone) DO NOTHING
    RETURNING id INTO v_user_id;

    -- Get the user ID
    IF v_user_id IS NULL THEN
        SELECT id INTO v_user_id FROM users WHERE phone = '0771234567';
    END IF;

    -- Create bus owner
    INSERT INTO bus_owners (
        user_id,
        company_name,
        identity_or_incorporation_no,
        business_email,
        business_phone,
        city,
        verification_status,
        profile_completed
    ) VALUES (
        v_user_id,
        'ABC Transport Services',
        'PV-2023-1234',
        'info@abctransport.lk',
        '0112345678',
        'Colombo',
        'verified',
        FALSE  -- Will be set to TRUE when first permit added
    )
    ON CONFLICT (user_id) DO NOTHING
    RETURNING id INTO v_bus_owner_id;

    -- Get the bus owner ID
    IF v_bus_owner_id IS NULL THEN
        SELECT id INTO v_bus_owner_id FROM bus_owners WHERE user_id = v_user_id;
    END IF;

    -- Insert sample permits
    INSERT INTO route_permits (
        bus_owner_id,
        permit_number,
        bus_registration_number,
        route_number,
        route_name,
        full_origin_city,
        full_destination_city,
        via,
        total_distance_km,
        estimated_duration_minutes,
        issue_date,
        expiry_date,
        permit_type,
        approved_fare,
        max_trips_per_day,
        allowed_bus_types,
        status,
        verified_at
    ) VALUES
    (
        v_bus_owner_id,
        'PERMIT-2025-001',
        'WP CAB-1234',
        '138',
        'Colombo - Kandy Express',
        'Colombo',
        'Kandy',
        ARRAY['Kadawatha', 'Kegalle'],
        115.0,
        180,
        '2024-01-01',
        '2025-12-31',
        'express',
        250.00,
        10,
        ARRAY['ac', 'deluxe', 'semi_sleeper'],
        'verified',
        NOW()
    ),
    (
        v_bus_owner_id,
        'PERMIT-2025-002',
        'WP CAB-5678',
        '177',
        'Colombo - Galle Highway',
        'Colombo',
        'Galle',
        ARRAY['Panadura', 'Kalutara', 'Aluthgama'],
        119.0,
        150,
        '2024-01-01',
        '2025-12-31',
        'regular',
        180.00,
        15,
        ARRAY['standard', 'ac', 'non_ac'],
        'verified',
        NOW()
    ),
    (
        v_bus_owner_id,
        'PERMIT-2025-003',
        'CP CAB-9012',
        '99',
        'Kandy - Nuwara Eliya Scenic',
        'Kandy',
        'Nuwara Eliya',
        ARRAY['Peradeniya', 'Gampola', 'Ramboda'],
        77.0,
        150,
        '2024-01-01',
        '2025-12-31',
        'regular',
        150.00,
        8,
        ARRAY['standard', 'deluxe'],
        'verified',
        NOW()
    )
    ON CONFLICT (bus_owner_id, permit_number) DO NOTHING;

END $$;
*/

-- ============================================================================
-- 7. MIGRATION ROLLBACK (if needed)
-- ============================================================================

-- To rollback this migration, run:
/*
DROP TRIGGER IF EXISTS auto_complete_profile ON route_permits;
DROP FUNCTION IF EXISTS update_bus_owner_profile_completed();
DROP FUNCTION IF EXISTS is_permit_expiring_soon(UUID);
DROP FUNCTION IF EXISTS is_permit_valid(UUID);
DROP TABLE IF EXISTS route_permit_stops CASCADE;
DROP TABLE IF EXISTS route_permits CASCADE;
ALTER TABLE bus_owners DROP COLUMN IF EXISTS profile_completed;
ALTER TABLE bus_owners DROP COLUMN IF EXISTS identity_or_incorporation_no;
-- Optionally restore: ALTER TABLE bus_owners ADD COLUMN license_number VARCHAR(100) UNIQUE;
*/

-- ============================================================================
-- MIGRATION COMPLETE
-- ============================================================================
