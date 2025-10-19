-- ============================================================================
-- Quick Mock Data for User 0779999999
-- Run this in Supabase SQL Editor
-- ============================================================================

-- Update the existing user 0779999999 to have complete profile data
UPDATE users
SET 
    first_name = 'Test',
    last_name = 'User',
    email = 'testuser@example.com',
    address = '123 Test Street, Colombo 07',
    city = 'Colombo',
    postal_code = '00700',
    profile_completed = true,
    email_verified = true,
    last_login_at = NOW(),
    updated_at = NOW()
WHERE phone = '0779999999';

-- Verify the update
SELECT 
    id,
    phone,
    first_name,
    last_name,
    email,
    city,
    roles,
    profile_completed,
    phone_verified,
    email_verified,
    status,
    created_at
FROM users
WHERE phone = '0779999999';

-- ============================================================================
-- Or if you want to add NIC and date of birth too:
-- ============================================================================

-- Uncomment and run this for more complete profile:
/*
UPDATE users
SET 
    first_name = 'Test',
    last_name = 'User',
    email = 'testuser@example.com',
    nic = '199512345678',
    date_of_birth = '1995-06-15',
    address = '123 Test Street, Colombo 07',
    city = 'Colombo',
    postal_code = '00700',
    profile_completed = true,
    email_verified = true,
    last_login_at = NOW(),
    updated_at = NOW()
WHERE phone = '0779999999';
*/

-- ============================================================================
-- Add a refresh token for this user (active session)
-- ============================================================================

INSERT INTO refresh_tokens (
    id,
    user_id,
    token_hash,
    device_id,
    device_type,
    ip_address,
    user_agent,
    created_at,
    expires_at,
    last_used_at,
    revoked
)
SELECT 
    gen_random_uuid(),
    id,
    encode(sha256('test_refresh_token_12345'::bytea), 'hex'),
    'test-device-android',
    'android',
    '::1',
    'Test User Agent',
    NOW(),
    NOW() + INTERVAL '7 days',
    NOW(),
    false
FROM users 
WHERE phone = '0779999999'
ON CONFLICT DO NOTHING;

-- ============================================================================
-- Success message
-- ============================================================================
SELECT 
    '✅ User 0779999999 updated with mock data!' as message,
    first_name || ' ' || last_name as full_name,
    email,
    city,
    profile_completed
FROM users 
WHERE phone = '0779999999';
