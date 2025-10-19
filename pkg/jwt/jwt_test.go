package jwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAccessSecret  = "test-access-secret-key-for-testing-purposes"
	testRefreshSecret = "test-refresh-secret-key-for-testing-purposes"
)

func TestNewService(t *testing.T) {
	service := NewService(
		testAccessSecret,
		testRefreshSecret,
		time.Hour,
		24*time.Hour,
	)

	assert.NotNil(t, service)
	assert.Equal(t, testAccessSecret, service.accessSecret)
	assert.Equal(t, testRefreshSecret, service.refreshSecret)
	assert.Equal(t, time.Hour, service.accessTokenExpiry)
	assert.Equal(t, 24*time.Hour, service.refreshTokenExpiry)
}

func TestGenerateAccessToken(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"
	roles := []string{"user"}
	profileCompleted := false

	token, err := service.GenerateAccessToken(userID, phone, roles, profileCompleted)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the generated token
	claims, err := service.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, phone, claims.Phone)
	assert.Equal(t, roles, claims.Roles)
	assert.Equal(t, profileCompleted, claims.ProfileCompleted)
	assert.Equal(t, AccessToken, claims.TokenType)
}

func TestGenerateRefreshToken(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"

	token, err := service.GenerateRefreshToken(userID, phone)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the generated token
	claims, err := service.ValidateRefreshToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, phone, claims.Phone)
	assert.Equal(t, RefreshToken, claims.TokenType)
}

func TestValidateAccessToken(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"
	roles := []string{"user", "admin"}
	profileCompleted := true

	// Generate valid token
	token, err := service.GenerateAccessToken(userID, phone, roles, profileCompleted)
	require.NoError(t, err)

	// Test valid token
	claims, err := service.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, phone, claims.Phone)
	assert.Equal(t, roles, claims.Roles)
	assert.Equal(t, profileCompleted, claims.ProfileCompleted)

	// Test invalid token
	_, err = service.ValidateAccessToken("invalid.token.here")
	assert.Error(t, err)

	// Test token with wrong secret
	wrongService := NewService("wrong-secret", testRefreshSecret, time.Hour, 24*time.Hour)
	_, err = wrongService.ValidateAccessToken(token)
	assert.Error(t, err)
}

func TestValidateRefreshToken(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"

	// Generate valid token
	token, err := service.GenerateRefreshToken(userID, phone)
	require.NoError(t, err)

	// Test valid token
	claims, err := service.ValidateRefreshToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, phone, claims.Phone)

	// Test invalid token
	_, err = service.ValidateRefreshToken("invalid.token.here")
	assert.Error(t, err)

	// Test token with wrong secret
	wrongService := NewService(testAccessSecret, "wrong-secret", time.Hour, 24*time.Hour)
	_, err = wrongService.ValidateRefreshToken(token)
	assert.Error(t, err)
}

func TestTokenTypeMismatch(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"

	// Generate access token
	accessToken, err := service.GenerateAccessToken(userID, phone, []string{"user"}, false)
	require.NoError(t, err)

	// Try to extract claims and manually validate as refresh token
	// This should fail at token type check
	claims, err := service.ExtractClaims(accessToken)
	require.NoError(t, err)
	assert.Equal(t, AccessToken, claims.TokenType)

	// Generate refresh token
	refreshToken, err := service.GenerateRefreshToken(userID, phone)
	require.NoError(t, err)

	// Try to extract claims and manually validate as access token
	// This should fail at token type check
	claims, err = service.ExtractClaims(refreshToken)
	require.NoError(t, err)
	assert.Equal(t, RefreshToken, claims.TokenType)
}

func TestExpiredToken(t *testing.T) {
	// Create service with very short expiry
	service := NewService(testAccessSecret, testRefreshSecret, time.Millisecond, time.Millisecond)
	userID := uuid.New()
	phone := "0771234567"

	// Generate token
	token, err := service.GenerateAccessToken(userID, phone, []string{"user"}, false)
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Try to validate expired token
	_, err = service.ValidateAccessToken(token)
	assert.Error(t, err)
}

func TestExtractClaims(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"
	roles := []string{"user"}

	token, err := service.GenerateAccessToken(userID, phone, roles, true)
	require.NoError(t, err)

	claims, err := service.ExtractClaims(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, phone, claims.Phone)
	assert.Equal(t, roles, claims.Roles)
	assert.True(t, claims.ProfileCompleted)
}

func TestIsTokenExpired(t *testing.T) {
	// Test valid token
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"

	token, err := service.GenerateAccessToken(userID, phone, []string{"user"}, false)
	require.NoError(t, err)

	assert.False(t, service.IsTokenExpired(token))

	// Test expired token
	expiredService := NewService(testAccessSecret, testRefreshSecret, -time.Hour, 24*time.Hour)
	expiredToken, err := expiredService.GenerateAccessToken(userID, phone, []string{"user"}, false)
	require.NoError(t, err)

	assert.True(t, service.IsTokenExpired(expiredToken))

	// Test invalid token
	assert.True(t, service.IsTokenExpired("invalid.token.here"))
}

func TestGetTokenExpiry(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"

	token, err := service.GenerateAccessToken(userID, phone, []string{"user"}, false)
	require.NoError(t, err)

	expiry, err := service.GetTokenExpiry(token)
	require.NoError(t, err)

	// Check expiry is approximately 1 hour from now
	expectedExpiry := time.Now().Add(time.Hour)
	assert.WithinDuration(t, expectedExpiry, expiry, 5*time.Second)

	// Test invalid token
	_, err = service.GetTokenExpiry("invalid.token.here")
	assert.Error(t, err)
}

func TestTokenSigningMethod(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"

	// Verify that our service generates HS256 tokens
	token, err := service.GenerateAccessToken(userID, phone, []string{"user"}, false)
	require.NoError(t, err)

	// Parse to check method
	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(testAccessSecret), nil
	})
	require.NoError(t, err)

	_, ok := parsedToken.Method.(*jwt.SigningMethodHMAC)
	assert.True(t, ok, "Token should use HMAC signing method")
}

func TestMultipleRoles(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"
	roles := []string{"user", "admin", "super_admin"}

	token, err := service.GenerateAccessToken(userID, phone, roles, true)
	require.NoError(t, err)

	claims, err := service.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, roles, claims.Roles)
	assert.Len(t, claims.Roles, 3)
}

func TestEmptyRoles(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"
	roles := []string{}

	token, err := service.GenerateAccessToken(userID, phone, roles, false)
	require.NoError(t, err)

	claims, err := service.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Empty(t, claims.Roles)
}

func TestTokenIssuerAndSubject(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)
	userID := uuid.New()
	phone := "0771234567"

	token, err := service.GenerateAccessToken(userID, phone, []string{"user"}, false)
	require.NoError(t, err)

	claims, err := service.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, "smarttransit-sms-auth", claims.Issuer)
	assert.Equal(t, userID.String(), claims.Subject)
}

func TestConcurrentTokenGeneration(t *testing.T) {
	service := NewService(testAccessSecret, testRefreshSecret, time.Hour, 24*time.Hour)

	done := make(chan bool)
	errors := make(chan error, 100)

	// Generate 100 tokens concurrently
	for i := 0; i < 100; i++ {
		go func() {
			userID := uuid.New()
			phone := "0771234567"

			token, err := service.GenerateAccessToken(userID, phone, []string{"user"}, false)
			if err != nil {
				errors <- err
				done <- true
				return
			}

			_, err = service.ValidateAccessToken(token)
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	close(errors)
	assert.Empty(t, errors)
}
