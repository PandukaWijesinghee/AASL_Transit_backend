package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// StaffHandler handles staff-related HTTP requests
type StaffHandler struct {
	staffService      *services.StaffService
	userRepo          *database.UserRepository
	staffRepo         *database.BusStaffRepository
	scheduledTripRepo *database.ScheduledTripRepository
}

// NewStaffHandler creates a new StaffHandler
func NewStaffHandler(
	staffService *services.StaffService,
	userRepo *database.UserRepository,
	staffRepo *database.BusStaffRepository,
	scheduledTripRepo *database.ScheduledTripRepository,
) *StaffHandler {
	return &StaffHandler{
		staffService:      staffService,
		userRepo:          userRepo,
		staffRepo:         staffRepo,
		scheduledTripRepo: scheduledTripRepo,
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
		"first_name":        staff.FirstName,
		"last_name":         staff.LastName,
		"profile_completed": staff.ProfileCompleted,
		"is_employed":       false, // New registrations are not yet employed
		"next_step":         "Wait for a bus owner to link you or search for a bus owner to join",
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

// GetMyTrips gets trips assigned to the authenticated staff member
// GET /api/v1/staff/my-trips?start_date=2024-01-01&end_date=2024-01-31
func (h *StaffHandler) GetMyTrips(c *gin.Context) {
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

	// Get staff profile to get staff_id
	staff, err := h.staffRepo.GetByUserID(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_staff",
			"message": "User is not registered as staff",
		})
		return
	}

	// Parse date parameters (default to today + 7 days)
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var startDate, endDate time.Time

	if startDateStr == "" {
		// Default to today
		startDate = time.Now().Truncate(24 * time.Hour)
	} else {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_date",
				"message": "Invalid start_date format. Use YYYY-MM-DD",
			})
			return
		}
	}

	if endDateStr == "" {
		// Default to 7 days from start
		endDate = startDate.Add(7 * 24 * time.Hour)
	} else {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_date",
				"message": "Invalid end_date format. Use YYYY-MM-DD",
			})
			return
		}
	}

	// Get assigned trips
	trips, err := h.scheduledTripRepo.GetAssignedTripsForStaff(staff.ID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "fetch_failed",
			"message": "Failed to fetch assigned trips",
		})
		return
	}

	// Enrich trips with role information (is_driver, is_conductor)
	type TripWithRole struct {
		models.ScheduledTripWithRouteInfo
		IsDriver    bool `json:"is_driver"`
		IsConductor bool `json:"is_conductor"`
	}

	enrichedTrips := make([]TripWithRole, len(trips))
	for i, trip := range trips {
		enrichedTrips[i] = TripWithRole{
			ScheduledTripWithRouteInfo: trip,
			IsDriver:                   trip.AssignedDriverID != nil && *trip.AssignedDriverID == staff.ID,
			IsConductor:                trip.AssignedConductorID != nil && *trip.AssignedConductorID == staff.ID,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"trips":      enrichedTrips,
		"count":      len(enrichedTrips),
		"staff_id":   staff.ID,
		"staff_type": staff.StaffType,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	})
}
