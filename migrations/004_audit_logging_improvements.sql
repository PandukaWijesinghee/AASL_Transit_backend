-- ============================================================================
-- Migration: 004 - Audit Logging and IP/UA Improvements
-- ============================================================================
-- Date: October 19, 2025
-- Description: This migration ensures all tables have proper columns for
--              IP address and user agent tracking, and verifies audit logging
--              infrastructure is in place.
--
-- Changes:
-- 1. Verify otp_verifications has ip_address and user_agent columns
-- 2. Verify otp_rate_limits has identifier and identifier_type columns
-- 3. Add indexes for better audit log query performance
-- ============================================================================

-- Check if otp_verifications columns exist (they should from schema)
-- If not, add them
DO $$
BEGIN
    -- Add ip_address column if it doesn't exist
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'otp_verifications'
        AND column_name = 'ip_address'
    ) THEN
        ALTER TABLE otp_verifications ADD COLUMN ip_address VARCHAR(45);
        RAISE NOTICE 'Added ip_address column to otp_verifications';
    ELSE
        RAISE NOTICE 'ip_address column already exists in otp_verifications';
    END IF;

    -- Add user_agent column if it doesn't exist
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'otp_verifications'
        AND column_name = 'user_agent'
    ) THEN
        ALTER TABLE otp_verifications ADD COLUMN user_agent TEXT;
        RAISE NOTICE 'Added user_agent column to otp_verifications';
    ELSE
        RAISE NOTICE 'user_agent column already exists in otp_verifications';
    END IF;
END $$;

-- Check if otp_rate_limits columns exist (should be in updatedDB.sql)
DO $$
BEGIN
    -- Add identifier column if it doesn't exist
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'otp_rate_limits'
        AND column_name = 'identifier'
    ) THEN
        ALTER TABLE otp_rate_limits ADD COLUMN identifier VARCHAR(255) NOT NULL DEFAULT '';
        RAISE NOTICE 'Added identifier column to otp_rate_limits';
    ELSE
        RAISE NOTICE 'identifier column already exists in otp_rate_limits';
    END IF;

    -- Add identifier_type column if it doesn't exist
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'otp_rate_limits'
        AND column_name = 'identifier_type'
    ) THEN
        ALTER TABLE otp_rate_limits ADD COLUMN identifier_type VARCHAR(10) NOT NULL DEFAULT 'phone';
        RAISE NOTICE 'Added identifier_type column to otp_rate_limits';
    ELSE
        RAISE NOTICE 'identifier_type column already exists in otp_rate_limits';
    END IF;
END $$;

-- Add indexes for audit_logs to improve query performance
-- These indexes help when querying recent events by user or action
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created
    ON audit_logs(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created
    ON audit_logs(action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_ip_created
    ON audit_logs(ip_address, created_at DESC);

-- Add index for otp_verifications to query by IP (fraud detection)
CREATE INDEX IF NOT EXISTS idx_otp_verifications_ip
    ON otp_verifications(ip_address, created_at DESC);

-- Add index for otp_rate_limits identifier lookups
CREATE INDEX IF NOT EXISTS idx_otp_rate_limits_identifier
    ON otp_rate_limits(identifier, identifier_type, created_at DESC);

-- ============================================================================
-- DATA MIGRATION (if needed)
-- ============================================================================

-- If there's old data in otp_rate_limits with 'phone' column but no 'identifier',
-- migrate it to the new structure
DO $$
BEGIN
    -- Check if old 'phone' column exists and new structure is in place
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'otp_rate_limits'
        AND column_name = 'phone'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'otp_rate_limits'
        AND column_name = 'identifier'
    ) THEN
        -- Migrate data from old 'phone' column to new 'identifier' column
        UPDATE otp_rate_limits
        SET identifier = phone, identifier_type = 'phone'
        WHERE identifier = '' OR identifier IS NULL;

        RAISE NOTICE 'Migrated phone data to identifier column';

        -- Optionally drop the old 'phone' column after migration
        -- ALTER TABLE otp_rate_limits DROP COLUMN IF EXISTS phone;
        -- RAISE NOTICE 'Dropped old phone column';
    END IF;
END $$;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

-- Verify all required columns exist
DO $$
DECLARE
    missing_columns TEXT := '';
BEGIN
    -- Check otp_verifications
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'otp_verifications' AND column_name = 'ip_address') THEN
        missing_columns := missing_columns || 'otp_verifications.ip_address, ';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'otp_verifications' AND column_name = 'user_agent') THEN
        missing_columns := missing_columns || 'otp_verifications.user_agent, ';
    END IF;

    -- Check otp_rate_limits
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'otp_rate_limits' AND column_name = 'identifier') THEN
        missing_columns := missing_columns || 'otp_rate_limits.identifier, ';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'otp_rate_limits' AND column_name = 'identifier_type') THEN
        missing_columns := missing_columns || 'otp_rate_limits.identifier_type, ';
    END IF;

    IF missing_columns != '' THEN
        RAISE EXCEPTION 'Migration failed! Missing columns: %', missing_columns;
    ELSE
        RAISE NOTICE '✓ All required columns exist';
        RAISE NOTICE '✓ Migration 004 completed successfully';
    END IF;
END $$;

-- ============================================================================
-- ROLLBACK (if needed)
-- ============================================================================

-- To rollback this migration, run:
-- DROP INDEX IF EXISTS idx_audit_logs_user_created;
-- DROP INDEX IF EXISTS idx_audit_logs_action_created;
-- DROP INDEX IF EXISTS idx_audit_logs_ip_created;
-- DROP INDEX IF EXISTS idx_otp_verifications_ip;
-- DROP INDEX IF EXISTS idx_otp_rate_limits_identifier;

-- NOTE: Do NOT drop the columns as they may contain data
-- Only drop indexes which can be recreated
