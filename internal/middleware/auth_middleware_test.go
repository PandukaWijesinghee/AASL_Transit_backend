package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestJWTService() *jwt.Service {
	return jwt.NewService(
		"test-access-secret-key-123456789",
		"test-refresh-secret-key-123456789",
		time.Hour,
		24*time.Hour,
	)
}

func setupTestRouter(jwtService *jwt.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func TestAuthMiddleware_Success(t *testing.T) {
	jwtService := setupTestJWTService()
	router := setupTestRouter(jwtService)

	userID := uuid.New()
	phone := "+94712345678"
	roles := []string{"passenger"}

	// Generate valid token
	token, err := jwtService.GenerateAccessToken(userID, phone, roles, true)
	require.NoError(t, err)

	// Setup protected route
	router.GET("/protected", AuthMiddleware(jwtService), func(c *gin.Context) {
		userCtx, exists := GetUserContext(c)
		require.True(t, exists)
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
			"user_id": userCtx.UserID,
			"phone":   userCtx.Phone,
		})
	})

	// Make request
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
	assert.Contains(t, w.Body.String(), phone)
}

func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	jwtService := setupTestJWTService()
	router := setupTestRouter(jwtService)

	router.GET("/protected", AuthMiddleware(jwtService), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header is required")
	assert.Contains(t, w.Body.String(), "MISSING_AUTH_HEADER")
}

func TestAuthMiddleware_InvalidAuthFormat(t *testing.T) {
	jwtService := setupTestJWTService()
	router := setupTestRouter(jwtService)

	router.GET("/protected", AuthMiddleware(jwtService), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
	})

	tests := []struct {
		name   string
		header string
	}{
		{"Missing Bearer", "some-token"},
		{"Wrong prefix", "Basic some-token"},
		{"Empty Bearer", "Bearer "},
		{"No token", "Bearer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "INVALID_AUTH_FORMAT")
		})
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	jwtService := setupTestJWTService()
	router := setupTestRouter(jwtService)

	router.GET("/protected", AuthMiddleware(jwtService), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
	})

	tests := []struct {
		name       string
		token      string
		expectCode string // Either INVALID_TOKEN or TOKEN_EXPIRED is acceptable
	}{
		{"Malformed token", "invalid.token.here", ""},       // Can be either
		{"Random string", "randomstringnotavalidtoken", ""}, // Can be either
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			// Accept either INVALID_TOKEN or TOKEN_EXPIRED error codes
			body := w.Body.String()
			hasValidError := strings.Contains(body, "INVALID_TOKEN") || strings.Contains(body, "TOKEN_EXPIRED")
			assert.True(t, hasValidError, "Expected INVALID_TOKEN or TOKEN_EXPIRED error, got: %s", body)
		})
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	// Create service with very short expiry
	jwtService := jwt.NewService(
		"test-access-secret-key-123456789",
		"test-refresh-secret-key-123456789",
		1*time.Millisecond, // Very short expiry
		24*time.Hour,
	)

	router := setupTestRouter(jwtService)

	userID := uuid.New()
	phone := "+94712345678"
	roles := []string{"passenger"}

	// Generate token
	token, err := jwtService.GenerateAccessToken(userID, phone, roles, true)
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	router.GET("/protected", AuthMiddleware(jwtService), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "TOKEN_EXPIRED")
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	jwtService := setupTestJWTService()

	// Create token with different secret
	wrongService := jwt.NewService(
		"wrong-secret-key",
		"wrong-refresh-secret",
		time.Hour,
		24*time.Hour,
	)

	userID := uuid.New()
	token, err := wrongService.GenerateAccessToken(userID, "+94712345678", []string{"passenger"}, true)
	require.NoError(t, err)

	router := setupTestRouter(jwtService)
	router.GET("/protected", AuthMiddleware(jwtService), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_TOKEN")
}

func TestGetUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	t.Run("Context exists", func(t *testing.T) {
		expectedCtx := UserContext{
			UserID:           uuid.New(),
			Phone:            "+94712345678",
			Roles:            []string{"passenger"},
			ProfileCompleted: true,
		}

		c.Set(UserContextKey, expectedCtx)

		userCtx, exists := GetUserContext(c)
		assert.True(t, exists)
		assert.Equal(t, expectedCtx.UserID, userCtx.UserID)
		assert.Equal(t, expectedCtx.Phone, userCtx.Phone)
		assert.Equal(t, expectedCtx.Roles, userCtx.Roles)
		assert.Equal(t, expectedCtx.ProfileCompleted, userCtx.ProfileCompleted)
	})

	t.Run("Context not found", func(t *testing.T) {
		c2, _ := gin.CreateTestContext(httptest.NewRecorder())
		userCtx, exists := GetUserContext(c2)
		assert.False(t, exists)
		assert.Equal(t, UserContext{}, userCtx)
	})

	t.Run("Context wrong type", func(t *testing.T) {
		c3, _ := gin.CreateTestContext(httptest.NewRecorder())
		c3.Set(UserContextKey, "wrong type")
		userCtx, exists := GetUserContext(c3)
		assert.False(t, exists)
		assert.Equal(t, UserContext{}, userCtx)
	})
}

func TestMustGetUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Context exists - no panic", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		expectedCtx := UserContext{
			UserID: uuid.New(),
			Phone:  "+94712345678",
		}
		c.Set(UserContextKey, expectedCtx)

		assert.NotPanics(t, func() {
			userCtx := MustGetUserContext(c)
			assert.Equal(t, expectedCtx.UserID, userCtx.UserID)
		})
	})

	t.Run("Context not found - panic", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		assert.Panics(t, func() {
			MustGetUserContext(c)
		})
	})
}

func TestRequireRole(t *testing.T) {
	jwtService := setupTestJWTService()
	router := setupTestRouter(jwtService)

	userID := uuid.New()
	phone := "+94712345678"

	t.Run("User has required role", func(t *testing.T) {
		roles := []string{"passenger", "driver"}
		token, err := jwtService.GenerateAccessToken(userID, phone, roles, true)
		require.NoError(t, err)

		router.GET("/driver-only", AuthMiddleware(jwtService), RequireRole("driver"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/driver-only", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("User doesn't have required role", func(t *testing.T) {
		roles := []string{"passenger"}
		token, err := jwtService.GenerateAccessToken(userID, phone, roles, true)
		require.NoError(t, err)

		router2 := setupTestRouter(jwtService)
		router2.GET("/admin-only", AuthMiddleware(jwtService), RequireRole("admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		req := httptest.NewRequest("GET", "/admin-only", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router2.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "INSUFFICIENT_PERMISSIONS")
	})

	t.Run("Multiple roles allowed", func(t *testing.T) {
		roles := []string{"driver"}
		token, err := jwtService.GenerateAccessToken(userID, phone, roles, true)
		require.NoError(t, err)

		router3 := setupTestRouter(jwtService)
		router3.GET("/multi-role", AuthMiddleware(jwtService), RequireRole("admin", "driver", "passenger"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/multi-role", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router3.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("No user context", func(t *testing.T) {
		router4 := setupTestRouter(jwtService)
		// Note: RequireRole without AuthMiddleware
		router4.GET("/no-auth", RequireRole("admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		req := httptest.NewRequest("GET", "/no-auth", nil)
		w := httptest.NewRecorder()

		router4.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "MISSING_USER_CONTEXT")
	})
}

func TestRequireProfileComplete(t *testing.T) {
	jwtService := setupTestJWTService()
	router := setupTestRouter(jwtService)

	userID := uuid.New()
	phone := "+94712345678"
	roles := []string{"passenger"}

	t.Run("Profile is complete", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(userID, phone, roles, true)
		require.NoError(t, err)

		router.GET("/complete-profile-required", AuthMiddleware(jwtService), RequireProfileComplete(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/complete-profile-required", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("Profile is incomplete", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(userID, phone, roles, false)
		require.NoError(t, err)

		router2 := setupTestRouter(jwtService)
		router2.GET("/complete-profile-required", AuthMiddleware(jwtService), RequireProfileComplete(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		req := httptest.NewRequest("GET", "/complete-profile-required", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router2.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "PROFILE_INCOMPLETE")
	})

	t.Run("No user context", func(t *testing.T) {
		router3 := setupTestRouter(jwtService)
		router3.GET("/no-auth", RequireProfileComplete(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		req := httptest.NewRequest("GET", "/no-auth", nil)
		w := httptest.NewRecorder()

		router3.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "MISSING_USER_CONTEXT")
	})
}

func TestAuthMiddleware_Integration(t *testing.T) {
	jwtService := setupTestJWTService()
	router := setupTestRouter(jwtService)

	// Simulate real user
	userID := uuid.New()
	phone := "+94712345678"
	roles := []string{"passenger", "driver"}

	token, err := jwtService.GenerateAccessToken(userID, phone, roles, true)
	require.NoError(t, err)

	// Setup multiple protected routes with different requirements
	router.GET("/profile", AuthMiddleware(jwtService), func(c *gin.Context) {
		userCtx := MustGetUserContext(c)
		c.JSON(http.StatusOK, gin.H{
			"user_id":           userCtx.UserID,
			"phone":             userCtx.Phone,
			"roles":             userCtx.Roles,
			"profile_completed": userCtx.ProfileCompleted,
		})
	})

	router.GET("/driver-dashboard",
		AuthMiddleware(jwtService),
		RequireRole("driver"),
		RequireProfileComplete(),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "driver dashboard"})
		})

	router.GET("/admin-panel",
		AuthMiddleware(jwtService),
		RequireRole("admin"),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "admin panel"})
		})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		checkBody      string
	}{
		{
			name:           "Access profile - success",
			path:           "/profile",
			expectedStatus: http.StatusOK,
			checkBody:      phone,
		},
		{
			name:           "Access driver dashboard - success (has role and complete profile)",
			path:           "/driver-dashboard",
			expectedStatus: http.StatusOK,
			checkBody:      "driver dashboard",
		},
		{
			name:           "Access admin panel - forbidden (no admin role)",
			path:           "/admin-panel",
			expectedStatus: http.StatusForbidden,
			checkBody:      "INSUFFICIENT_PERMISSIONS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkBody != "" {
				assert.Contains(t, w.Body.String(), tt.checkBody)
			}
		})
	}
}
