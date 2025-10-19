-- ============================================================================
-- Mock/Test Data for SMS Authentication System
-- Execute this in Supabase SQL Editor to populate test data
-- ============================================================================

-- Clean up existing test data (optional - comment out if you want to keep existing data)
-- DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE phone LIKE '077%');
-- DELETE FROM otp_verifications WHERE phone LIKE '077%';
-- DELETE FROM otp_rate_limits WHERE identifier LIKE '077%';
-- DELETE FROM users WHERE phone LIKE '077%';

-- ============================================================================
-- 1. INSERT TEST USERS
-- ============================================================================

-- User 1: New user (just created via API) - Profile NOT completed
-- This is the user that was just created: 0779999999
INSERT INTO users (
    id, 
    phone, 
    email, 
    first_name, 
    last_name, 
    roles, 
    profile_completed, 
    phone_verified, 
    status,
    created_at,
    updated_at
) VALUES (
    'ac2d090c-eb25-4c7a-af2a-cb420aa26299',  -- The actual UUID from your test
    '0779999999',
    NULL,  -- No email yet
    NULL,  -- No first name yet
    NULL,  -- No last name yet
    ARRAY['passenger']::TEXT[],
    false,  -- Profile NOT completed
    true,   -- Phone verified via OTP
    'active',
    NOW(),
    NOW()
) ON CONFLICT (id) DO UPDATE 
SET 
    phone = EXCLUDED.phone,
    updated_at = NOW();

-- User 2: Completed profile passenger
INSERT INTO users (
    id, 
    phone, 
    email, 
    first_name, 
    last_name,
    nic,
    address,
    city,
    postal_code,
    roles, 
    profile_completed, 
    phone_verified, 
    email_verified,
    status,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    '0771234567',
    'john.doe@example.com',
    'John',
    'Doe',
    '199512345678',
    '123 Main Street, Colombo 03',
    'Colombo',
    '00300',
    ARRAY['passenger']::TEXT[],
    true,   -- Profile completed
    true,   -- Phone verified
    true,   -- Email verified
    'active',
    NOW() - INTERVAL '30 days',  -- Created 30 days ago
    NOW() - INTERVAL '5 days'    -- Last updated 5 days ago
) ON CONFLICT (phone) DO UPDATE 
SET 
    email = EXCLUDED.email,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    updated_at = NOW();

-- User 3: Another passenger with profile
INSERT INTO users (
    id, 
    phone, 
    email, 
    first_name, 
    last_name,
    address,
    city,
    roles, 
    profile_completed, 
    phone_verified, 
    status,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    '0772223333',
    'jane.smith@example.com',
    'Jane',
    'Smith',
    '456 Galle Road, Mount Lavinia',
    'Mount Lavinia',
    ARRAY['passenger']::TEXT[],
    true,
    true,
    'active',
    NOW() - INTERVAL '60 days',
    NOW() - INTERVAL '10 days'
) ON CONFLICT (phone) DO UPDATE 
SET 
    email = EXCLUDED.email,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    updated_at = NOW();

-- User 4: Admin user
INSERT INTO users (
    id, 
    phone, 
    email, 
    first_name, 
    last_name,
    roles, 
    profile_completed, 
    phone_verified, 
    email_verified,
    status,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    '0777777777',
    'admin@smarttransit.lk',
    'Admin',
    'User',
    ARRAY['admin', 'passenger']::TEXT[],
    true,
    true,
    true,
    'active',
    NOW() - INTERVAL '90 days',
    NOW() - INTERVAL '1 day'
) ON CONFLICT (phone) DO UPDATE 
SET 
    email = EXCLUDED.email,
    roles = EXCLUDED.roles,
    updated_at = NOW();

-- User 5: Inactive user
INSERT INTO users (
    id, 
    phone, 
    email, 
    first_name, 
    last_name,
    roles, 
    profile_completed, 
    phone_verified, 
    status,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    '0765554444',
    'inactive@example.com',
    'Inactive',
    'User',
    ARRAY['passenger']::TEXT[],
    true,
    true,
    'inactive',  -- Inactive status
    NOW() - INTERVAL '120 days',
    NOW() - INTERVAL '90 days'
) ON CONFLICT (phone) DO UPDATE 
SET 
    status = 'inactive',
    updated_at = NOW();

-- ============================================================================
-- 2. INSERT TEST OTP VERIFICATIONS (Recent OTP History)
-- ============================================================================

-- Recent verified OTP for user 0779999999
INSERT INTO otp_verifications (
    phone,
    otp_code,
    purpose,
    created_at,
    expires_at,
    verified,
    verified_at,
    attempts,
    ip_address,
    user_agent
) VALUES
(
    '0779999999',
    '758794',  -- The OTP we just used
    'login',
    NOW() - INTERVAL '5 minutes',
    NOW() + INTERVAL '295 seconds',  -- 5 min expiry
    true,
    NOW() - INTERVAL '4 minutes',
    1,
    '::1',
    'Mozilla/5.0 (Windows NT; Windows NT 10.0; en-US) WindowsPowerShell'
);

-- Active (not yet verified) OTP for another user
INSERT INTO otp_verifications (
    phone,
    otp_code,
    purpose,
    created_at,
    expires_at,
    verified,
    attempts,
    ip_address
) VALUES
(
    '0771234567',
    '123456',
    'login',
    NOW() - INTERVAL '2 minutes',
    NOW() + INTERVAL '3 minutes',
    false,
    0,
    '192.168.1.100'
);

-- Expired OTP (for testing expiry)
INSERT INTO otp_verifications (
    phone,
    otp_code,
    purpose,
    created_at,
    expires_at,
    verified,
    attempts
) VALUES
(
    '0772223333',
    '999999',
    'login',
    NOW() - INTERVAL '10 minutes',
    NOW() - INTERVAL '5 minutes',  -- Already expired
    false,
    2
);

-- ============================================================================
-- 3. INSERT RATE LIMIT RECORDS (for testing rate limiting)
-- ============================================================================

-- Recent OTP requests for 0779999999 (shows rate limiting in action)
INSERT INTO otp_rate_limits (identifier, identifier_type, created_at)
VALUES
    ('0779999999', 'phone', NOW() - INTERVAL '8 minutes'),
    ('0779999999', 'phone', NOW() - INTERVAL '5 minutes'),
    ('0779999999', 'phone', NOW() - INTERVAL '2 minutes');

-- Some IP-based rate limits
INSERT INTO otp_rate_limits (identifier, identifier_type, created_at)
VALUES
    ('192.168.1.100', 'ip', NOW() - INTERVAL '6 minutes'),
    ('192.168.1.100', 'ip', NOW() - INTERVAL '3 minutes');

-- ============================================================================
-- 4. INSERT REFRESH TOKENS (Active sessions)
-- ============================================================================

-- Active session for user 0779999999
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
) VALUES (
    gen_random_uuid(),
    'ac2d090c-eb25-4c7a-af2a-cb420aa26299',  -- User 0779999999
    encode(sha256('refresh_token_for_0779999999'::bytea), 'hex'),
    'android-device-12345',
    'android',
    '::1',
    'Mozilla/5.0 (Windows NT; Windows NT 10.0; en-US) WindowsPowerShell',
    NOW(),
    NOW() + INTERVAL '7 days',
    NOW(),
    false
);

-- Active session for user 0771234567
INSERT INTO refresh_tokens (
    id,
    user_id,
    token_hash,
    device_id,
    device_type,
    ip_address,
    created_at,
    expires_at,
    last_used_at,
    revoked
) 
SELECT 
    gen_random_uuid(),
    id,
    encode(sha256('refresh_token_for_0771234567'::bytea), 'hex'),
    'ios-device-67890',
    'ios',
    '192.168.1.100',
    NOW() - INTERVAL '2 days',
    NOW() + INTERVAL '5 days',
    NOW() - INTERVAL '1 hour',
    false
FROM users WHERE phone = '0771234567';

-- Revoked session (old token)
INSERT INTO refresh_tokens (
    id,
    user_id,
    token_hash,
    device_type,
    created_at,
    expires_at,
    revoked,
    revoked_at
) 
SELECT 
    gen_random_uuid(),
    id,
    encode(sha256('old_revoked_token'::bytea), 'hex'),
    'web',
    NOW() - INTERVAL '10 days',
    NOW() + INTERVAL '4 days',
    true,
    NOW() - INTERVAL '3 days'
FROM users WHERE phone = '0771234567';

-- ============================================================================
-- 5. VERIFICATION QUERIES
-- ============================================================================

-- Check all test users
SELECT 
    phone,
    COALESCE(first_name, '(not set)') as first_name,
    COALESCE(last_name, '(not set)') as last_name,
    COALESCE(email, '(not set)') as email,
    roles,
    profile_completed,
    phone_verified,
    status,
    created_at
FROM users
WHERE phone LIKE '077%'
ORDER BY created_at DESC;

-- Check OTP verifications
SELECT 
    phone,
    otp_code,
    verified,
    attempts,
    created_at,
    expires_at,
    CASE 
        WHEN expires_at > NOW() THEN 'Valid'
        ELSE 'Expired'
    END as otp_status
FROM otp_verifications
WHERE phone LIKE '077%'
ORDER BY created_at DESC;

-- Check rate limits
SELECT 
    identifier,
    identifier_type,
    COUNT(*) as request_count,
    MIN(created_at) as first_request,
    MAX(created_at) as last_request,
    CASE 
        WHEN COUNT(*) >= 3 THEN 'Rate Limited'
        ELSE 'OK'
    END as status
FROM otp_rate_limits
WHERE created_at > NOW() - INTERVAL '10 minutes'
GROUP BY identifier, identifier_type
ORDER BY last_request DESC;

-- Check active sessions
SELECT 
    u.phone,
    u.first_name,
    rt.device_type,
    rt.created_at as session_started,
    rt.last_used_at,
    rt.expires_at,
    rt.revoked,
    CASE 
        WHEN rt.revoked THEN 'Revoked'
        WHEN rt.expires_at < NOW() THEN 'Expired'
        ELSE 'Active'
    END as session_status
FROM refresh_tokens rt
JOIN users u ON rt.user_id = u.id
WHERE u.phone LIKE '077%'
ORDER BY rt.created_at DESC;

-- ============================================================================
-- SUCCESS MESSAGE
-- ============================================================================
SELECT 
    '✅ Mock data inserted successfully!' as message,
    COUNT(*) as total_test_users
FROM users 
WHERE phone LIKE '077%';
