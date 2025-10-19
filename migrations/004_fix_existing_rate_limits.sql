-- ============================================================================
-- Migration 004: Fix Existing otp_rate_limits Table Structure
-- Purpose: Update otp_rate_limits to match what the Go code expects
-- Execute this in Supabase SQL Editor
-- ============================================================================

-- Drop the old table structure
DROP TABLE IF EXISTS otp_rate_limits CASCADE;

-- Create new table with correct structure
CREATE TABLE otp_rate_limits (
    id SERIAL PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL,
    identifier_type VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_otp_rate_limits_identifier ON otp_rate_limits(identifier, identifier_type);
CREATE INDEX idx_otp_rate_limits_created_at ON otp_rate_limits(created_at);

-- Success message
SELECT 'otp_rate_limits table fixed successfully!' as message;
