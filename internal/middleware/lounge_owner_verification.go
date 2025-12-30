package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
)

// RequireApprovedLoungeOwner checks if the lounge owner is approved
// Must be used after AuthMiddleware to have userCtx available
func RequireApprovedLoungeOwner(loungeOwnerRepo *database.LoungeOwnerRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user context (set by AuthMiddleware)
		userCtx, exists := GetUserContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "User context not found",
			})
			c.Abort()
			return
		}

		// Get lounge owner
		owner, err := loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
		if err != nil {
			log.Printf("ERROR: Failed to get lounge owner for verification check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to verify lounge owner status",
			})
			c.Abort()
			return
		}

		if owner == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "not_lounge_owner",
				"message": "Lounge owner account not found",
			})
			c.Abort()
			return
		}

		// Check verification status
		if owner.VerificationStatus != "approved" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "not_verified",
				"message":             "Your lounge owner account is not approved yet. Please wait for admin approval.",
				"verification_status": owner.VerificationStatus,
			})
			c.Abort()
			return
		}

		// Store owner info in context for handlers to use
		c.Set("lounge_owner_id", owner.ID)

		c.Next()
	}
}
