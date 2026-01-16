package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// RequireVerifiedBusOwner checks if the bus owner is verified
// Must be used after AuthMiddleware to have userCtx available
func RequireVerifiedBusOwner(busOwnerRepo *database.BusOwnerRepository) gin.HandlerFunc {
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

		// Get bus owner
		owner, err := busOwnerRepo.GetByUserID(userCtx.UserID.String())
		if err != nil {
			log.Printf("ERROR: Failed to get bus owner for verification check: %v", err)
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "not_bus_owner",
				"message": "Bus owner account not found",
			})
			c.Abort()
			return
		}

		if owner == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "not_bus_owner",
				"message": "Bus owner account not found",
			})
			c.Abort()
			return
		}

		// Check verification status
		if owner.VerificationStatus != models.VerificationVerified {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "not_verified",
				"message":             "Your bus owner account is not verified yet. Please wait for admin approval.",
				"code":                "ACCOUNT_NOT_VERIFIED",
				"verification_status": owner.VerificationStatus,
			})
			c.Abort()
			return
		}

		// Store owner info in context for handlers to use
		c.Set("bus_owner_id", owner.ID)
		c.Set("bus_owner", owner)

		c.Next()
	}
}
