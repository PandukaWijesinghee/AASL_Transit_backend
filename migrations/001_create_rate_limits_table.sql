-- ============================================================================
-- Quick Fix: Create otp_rate_limits table
-- Run this in your Supabase SQL Editor
-- ============================================================================

-- Create the rate limiting table
CREATE TABLE IF NOT EXISTS otp_rate_limits (
    id SERIAL PRIMARY KEY,
    phone VARCHAR(15) NOT NULL,
    request_count INTEGER DEFAULT 1,
    window_start TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    blocked_until TIMESTAMP WITH TIME ZONE,
    last_request_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_rate_limit_phone ON otp_rate_limits(phone);
CREATE INDEX IF NOT EXISTS idx_rate_limit_blocked ON otp_rate_limits(blocked_until);

-- Success message
SELECT 'otp_rate_limits table created successfully!' as message;
