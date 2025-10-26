package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type BusOwnerHandler struct {
	busOwnerRepo *database.BusOwnerRepository
	permitRepo   *database.RoutePermitRepository
	userRepo     *database.UserRepository
}

func NewBusOwnerHandler(busOwnerRepo *database.BusOwnerRepository, permitRepo *database.RoutePermitRepository, userRepo *database.UserRepository) *BusOwnerHandler {
	return &BusOwnerHandler{
		busOwnerRepo: busOwnerRepo,
		permitRepo:   permitRepo,
		userRepo:     userRepo,
	}
}

// GetProfile retrieves the bus owner profile
// GET /api/v1/bus-owner/profile
func (h *BusOwnerHandler) GetProfile(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	c.JSON(http.StatusOK, busOwner)
}

// CheckProfileStatus checks if bus owner has completed onboarding
// GET /api/v1/bus-owner/profile-status
func (h *BusOwnerHandler) CheckProfileStatus(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Count permits
	permitCount, err := h.permitRepo.CountPermits(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count permits"})
		return
	}

	// Check if company info is complete
	hasCompanyInfo := busOwner.CompanyName != nil &&
	                  busOwner.IdentityOrIncorporationNo != nil &&
	                  *busOwner.CompanyName != "" &&
	                  *busOwner.IdentityOrIncorporationNo != ""

	c.JSON(http.StatusOK, gin.H{
		"profile_completed": busOwner.ProfileCompleted,
		"permit_count":      permitCount,
		"has_company_info":  hasCompanyInfo,
	})
}

// CompleteOnboardingRequest represents the onboarding request payload
type CompleteOnboardingRequest struct {
	CompanyName              string                              `json:"company_name" binding:"required"`
	IdentityOrIncorporationNo string                             `json:"identity_or_incorporation_no" binding:"required"`
	BusinessEmail            *string                             `json:"business_email,omitempty"`
	Permits                  []models.CreateRoutePermitRequest   `json:"permits" binding:"required,min=1,dive"`
}

// CompleteOnboarding handles the complete onboarding process
// POST /api/v1/bus-owner/complete-onboarding
func (h *BusOwnerHandler) CompleteOnboarding(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request first (need company_name for bus_owner creation)
	var req CompleteOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate at least one permit
	if len(req.Permits) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one permit is required"})
		return
	}

	// Get or create bus owner record (with company_name)
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			// Bus owner record doesn't exist, create it with company info
			busOwner, err = h.busOwnerRepo.CreateWithCompany(
				userCtx.UserID.String(),
				req.CompanyName,
				req.IdentityOrIncorporationNo,
				req.BusinessEmail,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bus owner profile"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
			return
		}
	} else {
		// Bus owner exists - check if profile is already completed
		if busOwner.ProfileCompleted {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Profile already completed. Onboarding can only be done once.",
				"code": "PROFILE_ALREADY_COMPLETED",
			})
			return
		}

		// Profile exists but not completed - update company info
		err = h.busOwnerRepo.UpdateProfile(
			busOwner.ID,
			req.CompanyName,
			req.IdentityOrIncorporationNo,
			req.BusinessEmail,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}
	}

	// Create permits (trigger will auto-set profile_completed)
	createdPermits := make([]models.RoutePermit, 0, len(req.Permits))
	for _, permitReq := range req.Permits {
		permit, err := models.NewRoutePermitFromRequest(busOwner.ID, &permitReq)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = h.permitRepo.Create(permit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create permit: " + err.Error()})
			return
		}

		createdPermits = append(createdPermits, *permit)
	}

	// Update users table to mark profile as completed
	// NOTE: bus_owners.profile_completed is automatically set by database trigger,
	// but we need to also update users.profile_completed for consistency
	err = h.userRepo.SetProfileCompleted(userCtx.UserID, true)
	if err != nil {
		// Log error but don't fail the request - bus owner profile is already complete
		// In production, you'd log this properly
	}

	// Add "bus_owner" role to user's roles array
	// This ensures JWT tokens include the role for authorization
	err = h.userRepo.AddUserRole(userCtx.UserID, "bus_owner")
	if err != nil {
		// Log error but don't fail the request
		// Role might already exist (AddUserRole prevents duplicates)
	}

	// Fetch updated profile (should have profile_completed = true now)
	updatedProfile, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Onboarding completed successfully",
		"profile": updatedProfile,
		"permits": createdPermits,
	})
}
