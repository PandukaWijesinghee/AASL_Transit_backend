-- ============================================================================
-- Smart Transit System - SMS Authentication Database Schema
-- ============================================================================
-- Author: System Generated  
-- Date: October 13, 2025
-- Version: 2.0 (SMS OTP Authentication - No Firebase)
-- 
-- Changes from v1.0:
-- - Removed Firebase UID dependencies
-- - Added phone number as primary authentication
-- - Added OTP verification tables
-- - Added profile_completed flag
-- - Added JWT refresh token management
-- - Simplified user roles to TEXT[] array
-- ============================================================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- ENUMS (TYPE DEFINITIONS)
-- ============================================================================

-- User status
CREATE TYPE user_status AS ENUM (
    'active',           -- User can use the system
    'inactive',         -- Temporarily disabled
    'suspended',        -- Banned/suspended by admin
    'pending'           -- Waiting for profile completion
);

-- Verification status for business entities
CREATE TYPE verification_status AS ENUM (
    'pending',          -- Documents submitted, waiting review
    'verified',         -- Approved by admin
    'rejected',         -- Documents rejected
    'suspended'         -- Previously verified but now suspended
);

-- Staff type (for bus staff)
CREATE TYPE staff_type AS ENUM ('driver', 'conductor');

-- Employment status
CREATE TYPE employment_status AS ENUM (
    'active',           -- Currently working
    'inactive',         -- On leave or temporarily off
    'suspended',        -- Disciplinary suspension
    'terminated'        -- Employment ended
);

-- Background check status
CREATE TYPE background_check_status AS ENUM (
    'pending',          -- Check in progress
    'cleared',          -- Passed background check
    'failed'            -- Failed background check
);

-- Device type for sessions
CREATE TYPE device_type AS ENUM ('android', 'ios', 'web');

-- ============================================================================
-- MAIN USERS TABLE
-- ============================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone VARCHAR(15) UNIQUE NOT NULL,              -- Primary identifier (e.g., 0771234567)
    email VARCHAR(255) UNIQUE,                      -- Optional, collected in profile
    first_name VARCHAR(100),                        -- Required in profile completion
    last_name VARCHAR(100),                         -- Required in profile completion
    nic VARCHAR(20) UNIQUE,                         -- National Identity Card (optional)
    date_of_birth DATE,                             -- Optional, for age verification
    address TEXT,                                   -- Full address
    city VARCHAR(100),                              -- City name
    postal_code VARCHAR(20),                        -- Postal/ZIP code
    roles TEXT[] DEFAULT ARRAY['passenger']::TEXT[], -- Array: ['passenger', 'driver', 'bus_owner']
    profile_photo_url TEXT,                         -- URL to profile photo (Firebase Storage or S3)
    profile_completed BOOLEAN DEFAULT FALSE,        -- NEW: Track if mandatory profile is filled
    status user_status DEFAULT 'pending',           -- Account status
    phone_verified BOOLEAN DEFAULT FALSE,           -- Set to true after OTP verification
    email_verified BOOLEAN DEFAULT FALSE,           -- Optional email verification
    last_login_at TIMESTAMP WITH TIME ZONE,         -- Track last login time
    metadata JSONB,                                 -- Additional flexible data
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_users_phone ON users(phone);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_profile_completed ON users(profile_completed);
CREATE INDEX idx_users_roles ON users USING GIN(roles);
CREATE INDEX idx_users_nic ON users(nic);
CREATE INDEX idx_users_phone_verified ON users(phone_verified);

-- ============================================================================
-- OTP VERIFICATION TABLE
-- ============================================================================

CREATE TABLE otp_verifications (
    id SERIAL PRIMARY KEY,
    phone VARCHAR(15) NOT NULL,                     -- Phone number OTP sent to
    otp_code VARCHAR(6) NOT NULL,                   -- 6-digit OTP code
    purpose VARCHAR(50) DEFAULT 'login',            -- 'login', 'signup', 'password_reset', etc.
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,   -- OTP expiry (5 minutes from creation)
    verified BOOLEAN DEFAULT FALSE,                 -- Whether OTP was successfully verified
    verified_at TIMESTAMP WITH TIME ZONE,           -- When OTP was verified
    attempts INTEGER DEFAULT 0,                     -- Number of verification attempts
    max_attempts INTEGER DEFAULT 3,                 -- Maximum allowed attempts
    ip_address VARCHAR(45),                         -- Client IP for security
    user_agent TEXT                                 -- Client user agent
);

-- Indexes
CREATE INDEX idx_otp_phone ON otp_verifications(phone);
CREATE INDEX idx_otp_phone_code ON otp_verifications(phone, otp_code, verified);
CREATE INDEX idx_otp_expires_at ON otp_verifications(expires_at);
CREATE INDEX idx_otp_created_at ON otp_verifications(created_at);

-- Auto-delete expired OTPs (cleanup function)
CREATE OR REPLACE FUNCTION delete_expired_otps()
RETURNS void AS $$
BEGIN
    DELETE FROM otp_verifications 
    WHERE expires_at < NOW() - INTERVAL '1 hour'
    AND verified = FALSE;
END;
$$ LANGUAGE plpgsql;

-- Schedule cleanup (run every hour via cron job or scheduler)
-- COMMENT: Set up a cron job to call SELECT delete_expired_otps();

-- ============================================================================
-- OTP RATE LIMITING TABLE
-- ============================================================================

CREATE TABLE otp_rate_limits (
    id SERIAL PRIMARY KEY,
    phone VARCHAR(15) NOT NULL,                     -- Phone number
    request_count INTEGER DEFAULT 1,                -- Number of OTP requests in current window
    window_start TIMESTAMP WITH TIME ZONE DEFAULT NOW(), -- When the rate limit window started
    blocked_until TIMESTAMP WITH TIME ZONE,         -- If blocked, when block expires
    last_request_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_rate_limit_phone ON otp_rate_limits(phone);
CREATE INDEX idx_rate_limit_blocked ON otp_rate_limits(blocked_until);

-- Function to check and update rate limit
CREATE OR REPLACE FUNCTION check_otp_rate_limit(
    p_phone VARCHAR(15),
    p_max_requests INTEGER DEFAULT 3,
    p_window_minutes INTEGER DEFAULT 10
) RETURNS BOOLEAN AS $$
DECLARE
    v_record RECORD;
    v_window_start TIMESTAMP WITH TIME ZONE;
BEGIN
    -- Calculate window start time
    v_window_start := NOW() - (p_window_minutes || ' minutes')::INTERVAL;
    
    -- Get existing rate limit record
    SELECT * INTO v_record 
    FROM otp_rate_limits 
    WHERE phone = p_phone;
    
    -- Check if blocked
    IF v_record.blocked_until IS NOT NULL AND v_record.blocked_until > NOW() THEN
        RETURN FALSE; -- Still blocked
    END IF;
    
    -- If no record or window expired, create/reset
    IF v_record IS NULL OR v_record.window_start < v_window_start THEN
        INSERT INTO otp_rate_limits (phone, request_count, window_start, last_request_at)
        VALUES (p_phone, 1, NOW(), NOW())
        ON CONFLICT (phone) DO UPDATE 
        SET request_count = 1, 
            window_start = NOW(), 
            last_request_at = NOW(),
            blocked_until = NULL;
        RETURN TRUE;
    END IF;
    
    -- Check if limit exceeded
    IF v_record.request_count >= p_max_requests THEN
        -- Block for 1 hour
        UPDATE otp_rate_limits 
        SET blocked_until = NOW() + INTERVAL '1 hour'
        WHERE phone = p_phone;
        RETURN FALSE;
    END IF;
    
    -- Increment counter
    UPDATE otp_rate_limits 
    SET request_count = request_count + 1,
        last_request_at = NOW()
    WHERE phone = p_phone;
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- JWT REFRESH TOKENS TABLE
-- ============================================================================

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(256) NOT NULL UNIQUE,       -- SHA-256 hash of refresh token
    device_id VARCHAR(255),                         -- Device identifier
    device_type device_type,                        -- android, ios, web
    ip_address VARCHAR(45),                         -- IP when token was created
    user_agent TEXT,                                -- Browser/app user agent
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,   -- Refresh token expiry (7 days)
    last_used_at TIMESTAMP WITH TIME ZONE,          -- When token was last used
    revoked BOOLEAN DEFAULT FALSE,                  -- Manually revoked
    revoked_at TIMESTAMP WITH TIME ZONE             -- When token was revoked
);

-- Indexes
CREATE INDEX idx_refresh_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_token_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_expires_at ON refresh_tokens(expires_at);
CREATE INDEX idx_refresh_revoked ON refresh_tokens(revoked);

-- ============================================================================
-- USER SESSIONS TABLE (ACTIVE SESSIONS)
-- ============================================================================

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL,                -- Unique device identifier
    device_type device_type NOT NULL,               -- android, ios, web
    device_model VARCHAR(100),                      -- e.g., "iPhone 12", "Samsung Galaxy S21"
    app_version VARCHAR(20),                        -- App version (e.g., "1.2.3")
    os_version VARCHAR(20),                         -- OS version (e.g., "Android 12", "iOS 15.0")
    fcm_token VARCHAR(512),                         -- Firebase Cloud Messaging token (for push notifications)
    ip_address VARCHAR(45),                         -- Current IP address
    location_permission BOOLEAN DEFAULT FALSE,      -- Location access granted
    notification_permission BOOLEAN DEFAULT FALSE,  -- Notification access granted
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_active BOOLEAN DEFAULT TRUE,                 -- Session is active
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (user_id, device_id)                     -- One session per device per user
);

-- Indexes
CREATE INDEX idx_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_sessions_device_id ON user_sessions(device_id);
CREATE INDEX idx_sessions_fcm_token ON user_sessions(fcm_token);
CREATE INDEX idx_sessions_is_active ON user_sessions(is_active);
CREATE INDEX idx_sessions_last_activity ON user_sessions(last_activity_at);

-- ============================================================================
-- BUS OWNERS TABLE
-- ============================================================================

CREATE TABLE bus_owners (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    company_name VARCHAR(255) NOT NULL,
    license_number VARCHAR(100) UNIQUE,             -- Business license number
    contact_person VARCHAR(255),                    -- Primary contact person
    address TEXT,                                   -- Business address
    city VARCHAR(100),
    state VARCHAR(100),
    country VARCHAR(100) DEFAULT 'Sri Lanka',
    postal_code VARCHAR(20),
    verification_status verification_status DEFAULT 'pending',
    verification_documents JSONB,                   -- Array of document URLs
    business_email VARCHAR(255),
    business_phone VARCHAR(20),
    tax_id VARCHAR(50),                             -- Tax identification number
    bank_account_details JSONB,                     -- Encrypted bank details
    total_buses INTEGER DEFAULT 0,                  -- Number of buses owned
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_bus_owners_user_id ON bus_owners(user_id);
CREATE INDEX idx_bus_owners_verification_status ON bus_owners(verification_status);
CREATE INDEX idx_bus_owners_city ON bus_owners(city);
CREATE INDEX idx_bus_owners_company_name ON bus_owners(company_name);

-- ============================================================================
-- BUS STAFF TABLE (DRIVERS & CONDUCTORS)
-- ============================================================================

CREATE TABLE bus_staff (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    bus_owner_id UUID NOT NULL REFERENCES bus_owners(id) ON DELETE CASCADE,
    staff_type staff_type NOT NULL,                 -- 'driver' or 'conductor'
    license_number VARCHAR(100),                    -- Driver's license (for drivers)
    license_expiry_date DATE,                       -- License expiry
    license_document_url TEXT,                      -- Uploaded license photo URL
    experience_years INTEGER DEFAULT 0,             -- Years of experience
    emergency_contact VARCHAR(20),                  -- Emergency contact number
    emergency_contact_name VARCHAR(255),            -- Emergency contact name
    medical_certificate_expiry DATE,                -- Medical cert expiry
    medical_certificate_url TEXT,                   -- Medical cert document URL
    background_check_status background_check_status DEFAULT 'pending',
    background_check_document_url TEXT,             -- Background check report URL
    employment_status employment_status DEFAULT 'active',
    hire_date DATE NOT NULL,                        -- Date hired
    termination_date DATE,                          -- Date employment ended (if applicable)
    salary_amount DECIMAL(10,2),                    -- Monthly salary
    performance_rating DECIMAL(3,2) DEFAULT 5.00,   -- Rating out of 5.00
    total_trips_completed INTEGER DEFAULT 0,        -- Number of trips completed
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_performance_rating CHECK (performance_rating >= 0.00 AND performance_rating <= 5.00)
);

-- Indexes
CREATE INDEX idx_bus_staff_user_id ON bus_staff(user_id);
CREATE INDEX idx_bus_staff_owner_id ON bus_staff(bus_owner_id);
CREATE INDEX idx_bus_staff_type ON bus_staff(staff_type);
CREATE INDEX idx_bus_staff_employment_status ON bus_staff(employment_status);
CREATE INDEX idx_bus_staff_license_number ON bus_staff(license_number);

-- ============================================================================
-- LOUNGE OWNERS TABLE
-- ============================================================================

CREATE TABLE lounge_owners (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    business_name VARCHAR(255) NOT NULL,
    business_license VARCHAR(100) UNIQUE,           -- Business license number
    contact_person VARCHAR(255),                    -- Primary contact
    business_address TEXT,
    city VARCHAR(100),
    state VARCHAR(100),
    country VARCHAR(100) DEFAULT 'Sri Lanka',
    postal_code VARCHAR(20),
    verification_status verification_status DEFAULT 'pending',
    verification_documents JSONB,                   -- Array of document URLs
    business_email VARCHAR(255),
    business_phone VARCHAR(20),
    tax_id VARCHAR(50),                             -- Tax ID
    bank_account_details JSONB,                     -- Encrypted bank details
    total_lounges INTEGER DEFAULT 0,                -- Number of lounges operated
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_lounge_owners_user_id ON lounge_owners(user_id);
CREATE INDEX idx_lounge_owners_verification_status ON lounge_owners(verification_status);
CREATE INDEX idx_lounge_owners_city ON lounge_owners(city);
CREATE INDEX idx_lounge_owners_business_name ON lounge_owners(business_name);

-- ============================================================================
-- SECURITY & AUDIT LOG TABLE
-- ============================================================================

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,                   -- 'login', 'logout', 'otp_request', 'otp_verify', etc.
    entity_type VARCHAR(50),                        -- 'user', 'bus', 'trip', etc.
    entity_id UUID,                                 -- ID of the affected entity
    ip_address VARCHAR(45),                         -- IP address
    user_agent TEXT,                                -- Browser/app user agent
    details JSONB,                                  -- Additional context
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);

-- ============================================================================
-- HELPER FUNCTIONS & TRIGGERS
-- ============================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply update_updated_at trigger to tables
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_bus_owners_updated_at 
    BEFORE UPDATE ON bus_owners
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_bus_staff_updated_at 
    BEFORE UPDATE ON bus_staff
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_lounge_owners_updated_at 
    BEFORE UPDATE ON lounge_owners
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_sessions_updated_at 
    BEFORE UPDATE ON user_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to log audit events
CREATE OR REPLACE FUNCTION log_audit(
    p_user_id UUID,
    p_action VARCHAR(100),
    p_entity_type VARCHAR(50),
    p_entity_id UUID,
    p_ip_address VARCHAR(45),
    p_user_agent TEXT,
    p_details JSONB
) RETURNS void AS $$
BEGIN
    INSERT INTO audit_logs (user_id, action, entity_type, entity_id, ip_address, user_agent, details)
    VALUES (p_user_id, p_action, p_entity_type, p_entity_id, p_ip_address, p_user_agent, p_details);
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- SAMPLE DATA (FOR TESTING)
-- ============================================================================

-- Insert sample admin user
INSERT INTO users (id, phone, email, first_name, last_name, roles, profile_completed, status, phone_verified) 
VALUES 
    ('00000000-0000-0000-0000-000000000001', '0771234567', 'admin@smarttransit.lk', 'System', 'Admin', 
     ARRAY['admin']::TEXT[], true, 'active', true);

-- Insert sample passenger
INSERT INTO users (id, phone, email, first_name, last_name, roles, profile_completed, status, phone_verified) 
VALUES 
    ('00000000-0000-0000-0000-000000000002', '0779876543', 'passenger@example.com', 'John', 'Doe', 
     ARRAY['passenger']::TEXT[], true, 'active', true);

-- Insert sample bus owner
INSERT INTO users (id, phone, email, first_name, last_name, roles, profile_completed, status, phone_verified) 
VALUES 
    ('00000000-0000-0000-0000-000000000003', '0771111111', 'owner@example.com', 'Jane', 'Smith', 
     ARRAY['bus_owner']::TEXT[], true, 'active', true);

INSERT INTO bus_owners (id, user_id, company_name, license_number, contact_person, city, verification_status)
VALUES 
    ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000003', 
     'ABC Transport', 'LIC-12345', 'Jane Smith', 'Colombo', 'verified');

-- ============================================================================
-- VIEWS FOR CONVENIENCE
-- ============================================================================

-- View: Users with complete profile data
CREATE OR REPLACE VIEW v_complete_users AS
SELECT 
    u.id,
    u.phone,
    u.email,
    u.first_name,
    u.last_name,
    u.roles,
    u.profile_completed,
    u.status,
    u.phone_verified,
    u.last_login_at,
    u.created_at
FROM users u
WHERE u.profile_completed = true
  AND u.status = 'active'
  AND u.phone_verified = true;

-- View: Pending profile completions
CREATE OR REPLACE VIEW v_pending_profiles AS
SELECT 
    u.id,
    u.phone,
    u.email,
    u.first_name,
    u.last_name,
    u.profile_completed,
    u.created_at,
    EXTRACT(EPOCH FROM (NOW() - u.created_at))/3600 AS hours_since_signup
FROM users u
WHERE u.profile_completed = false
ORDER BY u.created_at DESC;

-- ============================================================================
-- COMMENTS & DOCUMENTATION
-- ============================================================================

COMMENT ON TABLE users IS 'Main users table - phone number is primary authentication method';
COMMENT ON COLUMN users.profile_completed IS 'Flag to track if user has completed mandatory profile form';
COMMENT ON COLUMN users.roles IS 'Array of user roles: passenger, driver, conductor, bus_owner, lounge_operator, admin';

COMMENT ON TABLE otp_verifications IS 'Stores OTP codes for SMS verification';
COMMENT ON COLUMN otp_verifications.expires_at IS 'OTP expires after 5 minutes';
COMMENT ON COLUMN otp_verifications.attempts IS 'Maximum 3 attempts allowed per OTP';

COMMENT ON TABLE otp_rate_limits IS 'Rate limiting table to prevent OTP spam (max 3 per 10 minutes)';

COMMENT ON TABLE refresh_tokens IS 'JWT refresh tokens for maintaining user sessions';
COMMENT ON COLUMN refresh_tokens.token_hash IS 'SHA-256 hash of the actual refresh token';
COMMENT ON COLUMN refresh_tokens.expires_at IS 'Refresh tokens expire after 7 days';

COMMENT ON TABLE audit_logs IS 'Security audit log for tracking all important actions';

-- ============================================================================
-- MAINTENANCE QUERIES (FOR REFERENCE)
-- ============================================================================

-- Cleanup expired OTPs (run periodically)
-- SELECT delete_expired_otps();

-- Check OTP rate limit for a phone number
-- SELECT check_otp_rate_limit('0771234567', 3, 10);

-- Revoke all refresh tokens for a user
-- UPDATE refresh_tokens SET revoked = true, revoked_at = NOW() WHERE user_id = '<uuid>';

-- Delete inactive sessions (no activity for 30 days)
-- DELETE FROM user_sessions WHERE last_activity_at < NOW() - INTERVAL '30 days';

-- Get users who haven't completed profile in 24 hours
-- SELECT * FROM v_pending_profiles WHERE hours_since_signup > 24;

-- ============================================================================
-- END OF SCHEMA
-- ============================================================================
