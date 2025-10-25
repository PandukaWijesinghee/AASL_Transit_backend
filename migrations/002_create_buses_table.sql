-- Migration: Create buses table
-- Description: Table for storing bus information registered by bus owners
-- Each bus is tied to a route permit (1:1 relationship)

-- Create buses table
CREATE TABLE IF NOT EXISTS buses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bus_owner_id UUID NOT NULL,
    permit_id UUID NOT NULL,
    bus_number VARCHAR(50) NOT NULL,
    license_plate VARCHAR(20) NOT NULL UNIQUE,
    bus_type VARCHAR(20) NOT NULL CHECK (bus_type IN ('normal', 'luxury', 'semi_luxury', 'super_luxury')),
    total_seats INTEGER NOT NULL CHECK (total_seats > 0),
    manufacturing_year INTEGER CHECK (manufacturing_year >= 1900 AND manufacturing_year <= EXTRACT(YEAR FROM CURRENT_DATE) + 1),
    last_maintenance_date DATE,
    insurance_expiry DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'maintenance', 'inactive')),

    -- Amenities (boolean flags)
    has_wifi BOOLEAN NOT NULL DEFAULT false,
    has_ac BOOLEAN NOT NULL DEFAULT false,
    has_charging_ports BOOLEAN NOT NULL DEFAULT false,
    has_entertainment BOOLEAN NOT NULL DEFAULT false,
    has_refreshments BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_bus_owner FOREIGN KEY (bus_owner_id) REFERENCES bus_owners(id) ON DELETE CASCADE,
    CONSTRAINT fk_permit FOREIGN KEY (permit_id) REFERENCES route_permits(id) ON DELETE RESTRICT
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_buses_bus_owner_id ON buses(bus_owner_id);
CREATE INDEX IF NOT EXISTS idx_buses_permit_id ON buses(permit_id);
CREATE INDEX IF NOT EXISTS idx_buses_license_plate ON buses(license_plate);
CREATE INDEX IF NOT EXISTS idx_buses_status ON buses(status);

-- Create unique constraint to ensure one bus per permit
CREATE UNIQUE INDEX IF NOT EXISTS idx_buses_permit_id_unique ON buses(permit_id);

-- Add comment to table
COMMENT ON TABLE buses IS 'Stores bus information registered by bus owners. Each bus is tied to one route permit.';
COMMENT ON COLUMN buses.bus_type IS 'Type of bus: normal, luxury, semi_luxury, super_luxury';
COMMENT ON COLUMN buses.status IS 'Current status: active, maintenance, inactive';
COMMENT ON CONSTRAINT fk_permit ON buses IS 'Ensures each bus is registered under a valid route permit. One permit = one bus.';
