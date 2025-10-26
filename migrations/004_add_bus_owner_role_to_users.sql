-- ============================================================================
-- Migration: Add "bus_owner" role to existing users who have bus_owner profiles
-- Version: 004
-- Description: Syncs users.roles with bus_owners table
-- ============================================================================

-- For NEW users going forward:
-- The CompleteOnboarding handler will automatically add "bus_owner" role

-- For EXISTING users who completed onboarding before this fix:
-- Update their roles array to include "bus_owner"

UPDATE users
SET
    roles = array_append(roles, 'bus_owner'),
    updated_at = NOW()
WHERE id IN (
    -- Find all users who have bus_owner profiles
    SELECT user_id
    FROM bus_owners
    WHERE profile_completed = true
)
-- Only add if they don't already have the role
AND NOT ('bus_owner' = ANY(roles));

-- Verify the update
SELECT
    '=== Users with bus_owner profiles ===' as status,
    u.id as user_id,
    u.phone,
    u.roles,
    bo.company_name,
    bo.profile_completed as bo_profile_completed,
    CASE
        WHEN 'bus_owner' = ANY(u.roles) THEN '✓ Has bus_owner role'
        ELSE '✗ Missing bus_owner role'
    END as role_status
FROM users u
JOIN bus_owners bo ON bo.user_id = u.id
WHERE bo.profile_completed = true
ORDER BY u.created_at DESC
LIMIT 20;

-- ============================================================================
-- SUMMARY
-- ============================================================================
SELECT '=== Migration 004 Complete ===' as status;
SELECT 'All existing bus owners now have "bus_owner" in their roles array' as result;
