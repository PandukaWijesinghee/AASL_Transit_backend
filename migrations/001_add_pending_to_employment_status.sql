-- ============================================================================
-- Migration: Add 'pending' to employment_status enum
-- Date: October 14, 2025
-- Purpose: Add pending status for driver/conductor registration flow
-- ============================================================================

-- Add 'pending' value to employment_status enum
-- This must be run separately and committed before it can be used
ALTER TYPE employment_status ADD VALUE IF NOT EXISTS 'pending';
