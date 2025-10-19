package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/pkg/jwt"
)

// UserContextKey is the key used to store user information in Gin context
const UserContextKey = "user"

// UserContext represents the authenticated user's information
type UserContext struct {
	UserID           uuid.UUID `json:"user_id"`
	Phone            string    `json:"phone"`
	Roles            []string  `json:"roles"`
	ProfileCompleted bool      `json:"profile_completed"`
}

// AuthMiddleware creates a middleware that validates JWT tokens
func AuthMiddleware(jwtService *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Printf("AUTH FAILED: Missing authorization header - Path: %s, IP: %s", c.Request.URL.Path, c.ClientIP())
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header is required",
				"code":    "MISSING_AUTH_HEADER",
			})
			c.Abort()
			return
		}

		// Check Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Printf("AUTH FAILED: Invalid auth format - Header: %s, Path: %s, IP: %s", authHeader[:20], c.Request.URL.Path, c.ClientIP())
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid authorization header format. Expected: Bearer <token>",
				"code":    "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(parts[1])

		// Check if token is empty
		if tokenString == "" {
			log.Printf("AUTH FAILED: Empty token - Path: %s, IP: %s", c.Request.URL.Path, c.ClientIP())
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Token cannot be empty",
				"code":    "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		// Validate token
		claims, err := jwtService.ValidateAccessToken(tokenString)
		if err != nil {
			// Check if token is expired
			if jwtService.IsTokenExpired(tokenString) {
				log.Printf("AUTH FAILED: Token expired - Path: %s, IP: %s, Error: %v", c.Request.URL.Path, c.ClientIP(), err)
				c.JSON(http.StatusUnauthorized, gin.H{
					"error":   "token_expired",
					"message": "Access token has expired. Please refresh your token.",
					"code":    "TOKEN_EXPIRED",
				})
			} else {
				log.Printf("AUTH FAILED: Invalid token - Path: %s, IP: %s, Error: %v", c.Request.URL.Path, c.ClientIP(), err)
				c.JSON(http.StatusUnauthorized, gin.H{
					"error":   "invalid_token",
					"message": "Invalid access token",
					"code":    "INVALID_TOKEN",
				})
			}
			c.Abort()
			return
		}

		// Create user context (UserID is already uuid.UUID type)
		userContext := UserContext{
			UserID:           claims.UserID,
			Phone:            claims.Phone,
			Roles:            claims.Roles,
			ProfileCompleted: claims.ProfileCompleted,
		}

		// Set user context in Gin context
		c.Set(UserContextKey, userContext)

		// Continue to next handler
		c.Next()
	}
}

// RequireRole creates a middleware that checks if user has required role
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user context
		userCtx, exists := GetUserContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "User context not found. Auth middleware may not be applied.",
				"code":    "MISSING_USER_CONTEXT",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userCtx.Roles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "You don't have permission to access this resource",
				"code":    "INSUFFICIENT_PERMISSIONS",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireProfileComplete creates a middleware that checks if profile is complete
func RequireProfileComplete() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user context
		userCtx, exists := GetUserContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "User context not found",
				"code":    "MISSING_USER_CONTEXT",
			})
			c.Abort()
			return
		}

		if !userCtx.ProfileCompleted {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "profile_incomplete",
				"message": "Please complete your profile to access this resource",
				"code":    "PROFILE_INCOMPLETE",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserContext retrieves the user context from Gin context
func GetUserContext(c *gin.Context) (UserContext, bool) {
	value, exists := c.Get(UserContextKey)
	if !exists {
		return UserContext{}, false
	}

	userCtx, ok := value.(UserContext)
	if !ok {
		return UserContext{}, false
	}

	return userCtx, true
}

// MustGetUserContext retrieves the user context or panics (use only after AuthMiddleware)
func MustGetUserContext(c *gin.Context) UserContext {
	userCtx, exists := GetUserContext(c)
	if !exists {
		panic("user context not found - ensure AuthMiddleware is applied")
	}
	return userCtx
}
