package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// StaffHandler handles staff-related HTTP requests
type StaffHandler struct {
	staffService *services.StaffService
	userRepo     *database.UserRepository
	staffRepo    *database.BusStaffRepository
}

// NewStaffHandler creates a new StaffHandler
func NewStaffHandler(
	staffService *services.StaffService,
	userRepo *database.UserRepository,
	staffRepo *database.BusStaffRepository,
) *StaffHandler {
	return &StaffHandler{
		staffService: staffService,
		userRepo:     userRepo,
		staffRepo:    staffRepo,
	}
}

// CheckRegistrationRequest represents check registration request
type CheckRegistrationRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

// CheckRegistration checks if user is registered as staff
// POST /api/v1/staff/check-registration
func (h *StaffHandler) CheckRegistration(c *gin.Context) {
	var req CheckRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	result, err := h.staffService.CheckStaffRegistration(req.PhoneNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "check_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RegisterStaff registers a new driver or conductor
// POST /api/v1/staff/register
func (h *StaffHandler) RegisterStaff(c *gin.Context) {
	var input models.StaffRegistrationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Validate driver-specific requirements
	if input.StaffType == models.StaffTypeDriver {
		// For now, license fields are optional at beginning
		// You can add stricter validation later
	}

	// Register staff
	staff, err := h.staffService.RegisterStaff(&input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "registration_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":           "Registration successful",
		"staff_id":          staff.ID,
		"staff_type":        staff.StaffType,
		"employment_status": staff.EmploymentStatus,
		"profile_completed": staff.ProfileCompleted,
		"requires_approval": true,
		"approval_message":  "Your registration is pending approval by the bus owner or admin",
	})
}

// GetProfile gets complete staff profile
// GET /api/v1/staff/profile
func (h *StaffHandler) GetProfile(c *gin.Context) {
	// Get user context from Gin (set by auth middleware)
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr := userCtx.UserID.String()
	profile, err := h.staffService.GetCompleteProfile(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "profile_not_found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateProfile updates staff profile
// PUT /api/v1/staff/profile
func (h *StaffHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "invalid_user_id",
			"message": "Invalid user ID format",
		})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	err := h.staffService.UpdateStaffProfile(userIDStr, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "update_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
	})
}

// SearchBusOwners searches for bus owners
// GET /api/v1/staff/bus-owners/search?code=ABC123
// GET /api/v1/staff/bus-owners/search?bus_number=WP-1234
func (h *StaffHandler) SearchBusOwners(c *gin.Context) {
	code := c.Query("code")
	busNumber := c.Query("bus_number")

	if code == "" && busNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Either code or bus_number is required",
		})
		return
	}

	var busOwner interface{}
	var err error

	if code != "" {
		busOwner, err = h.staffService.FindBusOwnerByCode(code)
	} else {
		busOwner, err = h.staffService.FindBusOwnerByBusNumber(busNumber)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"found":   false,
			"message": "Bus owner not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"found":     true,
		"bus_owner": busOwner,
	})
}
