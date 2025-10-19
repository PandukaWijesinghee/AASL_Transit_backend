-- Clear all data from application tables while preserving schema
-- Use with caution: this removes ALL rows and resets sequences

BEGIN;

TRUNCATE TABLE
    audit_logs,
    bus_staff,
    bus_owners,
    lounge_owners,
    otp_verifications,
    otp_rate_limits,
    refresh_tokens,
    user_sessions,
    users
RESTART IDENTITY CASCADE;

COMMIT;

-- Optional: show remaining row counts (should be zero)
-- SELECT 'audit_logs' AS table, COUNT(*) FROM audit_logs
-- UNION ALL SELECT 'bus_staff', COUNT(*) FROM bus_staff
-- UNION ALL SELECT 'bus_owners', COUNT(*) FROM bus_owners
-- UNION ALL SELECT 'lounge_owners', COUNT(*) FROM lounge_owners
-- UNION ALL SELECT 'otp_verifications', COUNT(*) FROM otp_verifications
-- UNION ALL SELECT 'otp_rate_limits', COUNT(*) FROM otp_rate_limits
-- UNION ALL SELECT 'refresh_tokens', COUNT(*) FROM refresh_tokens
-- UNION ALL SELECT 'user_sessions', COUNT(*) FROM user_sessions
-- UNION ALL SELECT 'users', COUNT(*) FROM users;
