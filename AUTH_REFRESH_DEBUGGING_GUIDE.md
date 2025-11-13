# Auth/Refresh Endpoint 401 Error Debugging Guide

## Issue Summary
The `/api/v1/auth/refresh` endpoint is returning 401 Unauthorized errors when the Flutter lounge owner app attempts to refresh access tokens.

## Backend Endpoint Details

### Route Configuration
```go
// From cmd/server/main.go line 325-326
auth.POST("/refresh-token", authHandler.RefreshToken)
auth.POST("/refresh", authHandler.RefreshToken) // Alias for mobile compatibility
```

**Important Notes:**
- ✅ The refresh endpoint is **NOT** protected by authentication middleware
- ✅ It's a public route that accepts refresh tokens in the request body
- ✅ Bearer tokens in Authorization headers are optional/ignored

### Request Format
```json
POST /api/v1/auth/refresh-token
Content-Type: application/json

{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "device_id": "optional-device-id",
  "device_type": "optional-device-type"
}
```

### Response Format (Success)
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in_seconds": 3600,
  "token_type": "Bearer"
}
```

## Possible Failure Points

The RefreshToken handler (internal/handlers/auth_handler.go:1122) has several validation steps that can return 401:

### 1. Invalid Request Body (400 - not 401)
```go
// Line 1124-1130
if err := c.ShouldBindJSON(&req); err != nil {
    return 400 BadRequest
}
```

### 2. Token Validation Failed (401) ⚠️ MOST LIKELY
```go
// Line 1132-1138
claims, err := h.jwtService.ValidateRefreshToken(req.RefreshToken)
if err != nil {
    return 401 "Invalid or expired refresh token"
}
```

**Causes:**
- Refresh token is expired (default: 7 days)
- Refresh token signature is invalid (JWT secret changed)
- Refresh token format is incorrect
- Token was not signed with the correct refresh secret

### 3. Token Revoked (401)
```go
// Line 1148-1154
if revoked {
    return 401 "Refresh token has been revoked"
}
```

**Causes:**
- Token was previously revoked (logout, token rotation)
- Token exists in database with revoked=true

### 4. User Not Found (401)
```go
// Line 1157-1165
if user == nil {
    return 401 "User no longer exists"
}
```

### 5. User Inactive (403 - not 401)
```go
// Line 1174-1178
if user.Status != "active" {
    return 403 "User account is not active"
}
```

## Enhanced Logging Added

New detailed logging has been added to help debug the issue:

```go
🔄 REFRESH TOKEN REQUEST: Token length: X, DeviceID: Y, DeviceType: Z
✅ REFRESH TOKEN: Token validated successfully for user: UUID, phone: PHONE
✅ REFRESH TOKEN: Token is not revoked, fetching user: UUID
✅ REFRESH TOKEN: User found - ID: UUID, Status: active
✅ REFRESH TOKEN SUCCESS: New tokens generated for user: UUID
```

Error logs:
```go
❌ REFRESH TOKEN ERROR: Invalid request body - ERROR
❌ REFRESH TOKEN ERROR: Token validation failed - ERROR
❌ REFRESH TOKEN ERROR: Failed to check if token is revoked - ERROR
❌ REFRESH TOKEN ERROR: Token has been revoked for user: UUID
❌ REFRESH TOKEN ERROR: Failed to fetch user UUID - ERROR
❌ REFRESH TOKEN ERROR: User UUID no longer exists
❌ REFRESH TOKEN ERROR: User UUID is not active, status: STATUS
```

## Flutter App Configuration

### API Client (lounge_owner_app/lib/core/network/api_client.dart)

The Flutter app's Dio interceptor automatically adds Bearer tokens to ALL requests:

```dart
// Line 27-35
onRequest: (options, handler) async {
  final tokens = await _authLocalDataSource.getTokens();
  if (tokens != null && !tokens.isExpired) {
    options.headers['Authorization'] = 'Bearer ${tokens.accessToken}';
  }
  return handler.next(options);
}
```

**Issue:** When refreshing tokens (line 65), the Bearer header is still added even though the endpoint doesn't need it.

```dart
// Line 65
final response = await _dio.post(
  '${AppConfig.authEndpoint}/refresh-token',
  data: {'refresh_token': tokens.refreshToken},
);
```

**Note:** This shouldn't cause issues since the backend ignores Bearer tokens on the refresh endpoint, but it's worth noting.

## Debugging Steps

### Step 1: Check Backend Logs
After deployment, monitor the logs for:
```
🔄 REFRESH TOKEN REQUEST: ...
```

Look for which error message appears:
- If you see "❌ REFRESH TOKEN ERROR: Token validation failed" → Token is invalid/expired
- If you see "✅ Token validated" but then "❌ Token has been revoked" → Token was revoked
- If you see "✅ Token is not revoked" but "❌ User no longer exists" → User was deleted

### Step 2: Check Token Expiry Configuration
```bash
# Check the backend environment variables
echo $JWT_REFRESH_TOKEN_EXPIRY  # Should be "168h" (7 days)
```

### Step 3: Check JWT Secrets
Ensure the JWT secrets haven't changed between deployments:
```bash
# These must be consistent across deployments
echo $JWT_ACCESS_SECRET
echo $JWT_REFRESH_SECRET
```

If secrets changed, all existing tokens are invalid.

### Step 4: Check Token Storage in Flutter
Add logging in `auth_local_datasource.dart` to verify:
- Refresh token is being stored correctly
- Refresh token is not empty or corrupted
- Token hasn't expired on the client side

### Step 5: Test Manually with cURL
```bash
# Get a fresh token by logging in
curl -X POST https://YOUR-API/api/v1/auth/send-otp \
  -H "Content-Type: application/json" \
  -d '{"phone_number": "+94710000999"}'

curl -X POST https://YOUR-API/api/v1/auth/verify-otp-lounge-owner \
  -H "Content-Type: application/json" \
  -d '{"phone_number": "+94710000999", "otp": "123456"}'

# Save the refresh_token from response, then test refresh
curl -X POST https://YOUR-API/api/v1/auth/refresh-token \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "YOUR_REFRESH_TOKEN_HERE"}'
```

### Step 6: Check Database
```sql
-- Check if token exists and is revoked
SELECT token, user_id, revoked, expires_at, last_used_at
FROM refresh_tokens
WHERE token = 'YOUR_REFRESH_TOKEN'
LIMIT 1;

-- Check user status
SELECT id, phone, status, roles
FROM users
WHERE id = 'USER_UUID';
```

## Common Solutions

### Solution 1: Token Expired
- **Symptom:** "Invalid or expired refresh token"
- **Fix:** User needs to log in again
- **Prevention:** Increase refresh token expiry time (currently 7 days)

### Solution 2: JWT Secrets Changed
- **Symptom:** All users getting 401 on refresh
- **Fix:** Ensure consistent JWT secrets in environment variables
- **Recovery:** All users must log in again

### Solution 3: Token Rotation Issues
- **Symptom:** First refresh works, second fails
- **Fix:** Check that new refresh token is being stored in Flutter app after successful refresh
- **Check:** Line 71-76 in api_client.dart should save new tokens

### Solution 4: Database Connection Issues
- **Symptom:** "Failed to verify token status" or "Failed to fetch user information"
- **Fix:** Check database connectivity and connection pool settings

## Next Steps

1. **Deploy Updated Backend:** The enhanced logging will help identify the exact failure point
2. **Monitor Logs:** Watch for the new log messages during token refresh attempts
3. **Check Token Expiry:** Verify tokens aren't expiring too quickly
4. **Verify JWT Secrets:** Ensure they're consistent across deployments
5. **Test End-to-End:** Try the full flow: login → use app → wait for token expiry → automatic refresh

## Related Files

### Backend
- `internal/handlers/auth_handler.go` - RefreshToken handler (line 1122)
- `pkg/jwt/jwt.go` - Token validation logic (line 105)
- `internal/database/refresh_token_repository.go` - Token storage/retrieval
- `cmd/server/main.go` - Route configuration (line 325)

### Flutter
- `lib/core/network/api_client.dart` - Dio interceptor and refresh logic (line 58)
- `lib/data/datasources/auth_local_datasource.dart` - Token storage
- `lib/data/repositories/auth_repository_impl.dart` - Auth operations (line 77)

## Token Lifecycle

```
1. User logs in → Access token (1h) + Refresh token (7d)
2. Access token expires → Dio interceptor detects 401
3. Interceptor calls _refreshToken() → POST /auth/refresh-token
4. Backend validates refresh token → Returns new access + refresh tokens
5. Old refresh token is revoked (token rotation)
6. New tokens stored in local storage
7. Original request is retried with new access token
```

## Important Notes

- ⚠️ Refresh tokens are single-use due to token rotation
- ⚠️ After successful refresh, the old refresh token is revoked
- ⚠️ The new refresh token must be stored immediately
- ⚠️ If the new token isn't stored, the next refresh will fail
- ⚠️ JWT secrets must remain constant across deployments
- ⚠️ Changing JWT secrets invalidates all existing tokens
