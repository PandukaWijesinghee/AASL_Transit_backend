-- ============================================================================
-- CORRECT otp_rate_limits Table Structure
-- This matches what the backend code expects
-- ============================================================================

-- Drop the old table if it exists (CAREFUL - this deletes data!)
DROP TABLE IF EXISTS otp_rate_limits;

-- Create the correct structure
CREATE TABLE otp_rate_limits (
    id SERIAL PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL,           -- Phone number or IP address
    identifier_type VARCHAR(20) NOT NULL,       -- 'phone' or 'ip'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_otp_rate_limits_identifier ON otp_rate_limits(identifier, identifier_type);
CREATE INDEX idx_otp_rate_limits_created_at ON otp_rate_limits(created_at);

-- Success message
SELECT 'otp_rate_limits table created successfully with correct structure!' as message;
