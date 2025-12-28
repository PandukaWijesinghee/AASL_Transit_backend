package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// ActiveTripHandler handles active trip HTTP requests
type ActiveTripHandler struct {
	activeTripService *services.ActiveTripService
	staffRepo         *database.BusStaffRepository
}

// NewActiveTripHandler creates a new ActiveTripHandler
func NewActiveTripHandler(
	activeTripService *services.ActiveTripService,
	staffRepo *database.BusStaffRepository,
) *ActiveTripHandler {
	return &ActiveTripHandler{
		activeTripService: activeTripService,
		staffRepo:         staffRepo,
	}
}

// StartTripRequest represents the request body for starting a trip
type StartTripRequest struct {
	ScheduledTripID  string  `json:"scheduled_trip_id" binding:"required"`
	InitialLatitude  float64 `json:"initial_latitude" binding:"required"`
	InitialLongitude float64 `json:"initial_longitude" binding:"required"`
}

// StartTrip starts a scheduled trip
// POST /api/v1/staff/trips/start
func (h *ActiveTripHandler) StartTrip(c *gin.Context) {
	// Get user context
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr := userCtx.UserID.String()

	// Get staff profile
	staff, err := h.staffRepo.GetByUserID(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_staff",
			"message": "User is not registered as staff",
		})
		return
	}

	// Parse request
	var req StartTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Start the trip
	result, err := h.activeTripService.StartTrip(&services.StartTripInput{
		ScheduledTripID:  req.ScheduledTripID,
		StaffID:          staff.ID,
		InitialLatitude:  req.InitialLatitude,
		InitialLongitude: req.InitialLongitude,
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "start_trip_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           result.Message,
		"active_trip":       result.ActiveTrip,
		"scheduled_trip_id": result.ScheduledTripID,
	})
}

// UpdateLocationRequest represents the request body for updating location
type UpdateLocationRequestBody struct {
	Latitude  float64  `json:"latitude" binding:"required"`
	Longitude float64  `json:"longitude" binding:"required"`
	SpeedKmh  *float64 `json:"speed_kmh,omitempty"`
	Heading   *float64 `json:"heading,omitempty"`
}

// UpdateLocation updates the current location of an active trip
// PUT /api/v1/staff/trips/:id/location
func (h *ActiveTripHandler) UpdateLocation(c *gin.Context) {
	// Get user context
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr := userCtx.UserID.String()

	// Get staff profile
	staff, err := h.staffRepo.GetByUserID(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_staff",
			"message": "User is not registered as staff",
		})
		return
	}

	// Get active trip ID from URL
	activeTripID := c.Param("id")
	if activeTripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_id",
			"message": "Active trip ID is required",
		})
		return
	}

	// Parse request
	var req UpdateLocationRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Update location
	err = h.activeTripService.UpdateLocation(&services.UpdateLocationInput{
		ActiveTripID: activeTripID,
		StaffID:      staff.ID,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		SpeedKmh:     req.SpeedKmh,
		Heading:      req.Heading,
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "update_location_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Location updated successfully",
	})
}

// EndTripRequest represents the request body for ending a trip
type EndTripRequest struct {
	FinalLatitude  float64 `json:"final_latitude" binding:"required"`
	FinalLongitude float64 `json:"final_longitude" binding:"required"`
}

// EndTrip completes an active trip
// POST /api/v1/staff/trips/:id/end
func (h *ActiveTripHandler) EndTrip(c *gin.Context) {
	// Get user context
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr := userCtx.UserID.String()

	// Get staff profile
	staff, err := h.staffRepo.GetByUserID(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_staff",
			"message": "User is not registered as staff",
		})
		return
	}

	// Get active trip ID from URL
	activeTripID := c.Param("id")
	if activeTripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_id",
			"message": "Active trip ID is required",
		})
		return
	}

	// Parse request
	var req EndTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// End the trip
	result, err := h.activeTripService.EndTrip(&services.EndTripInput{
		ActiveTripID:   activeTripID,
		StaffID:        staff.ID,
		FinalLatitude:  req.FinalLatitude,
		FinalLongitude: req.FinalLongitude,
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "end_trip_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     result.Message,
		"active_trip": result.ActiveTrip,
		"duration":    result.Duration,
	})
}

// GetActiveTrip retrieves an active trip by ID
// GET /api/v1/staff/trips/:id/active
func (h *ActiveTripHandler) GetActiveTrip(c *gin.Context) {
	// Get user context
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr := userCtx.UserID.String()

	// Get staff profile
	_, err := h.staffRepo.GetByUserID(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_staff",
			"message": "User is not registered as staff",
		})
		return
	}

	// Get active trip ID from URL
	activeTripID := c.Param("id")
	if activeTripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_id",
			"message": "Active trip ID is required",
		})
		return
	}

	// Get the active trip
	activeTrip, err := h.activeTripService.GetActiveTrip(activeTripID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "Active trip not found",
		})
		return
	}

	c.JSON(http.StatusOK, activeTrip)
}

// GetMyActiveTrip gets the current active trip for the authenticated staff member
// GET /api/v1/staff/trips/my-active
func (h *ActiveTripHandler) GetMyActiveTrip(c *gin.Context) {
	// Get user context
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr := userCtx.UserID.String()

	// Get staff profile
	staff, err := h.staffRepo.GetByUserID(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_staff",
			"message": "User is not registered as staff",
		})
		return
	}

	// Get my active trip
	activeTrip, err := h.activeTripService.GetMyActiveTrip(staff.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"has_active_trip": false,
			"message":         "No active trip",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_active_trip": true,
		"active_trip":     activeTrip,
	})
}

// UpdatePassengerCountRequest represents the request body for updating passenger count
type UpdatePassengerCountRequest struct {
	PassengerCount int `json:"passenger_count" binding:"required,min=0"`
}

// UpdatePassengerCount updates the passenger count for an active trip
// PUT /api/v1/staff/trips/:id/passengers
func (h *ActiveTripHandler) UpdatePassengerCount(c *gin.Context) {
	// Get user context
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User not authenticated",
		})
		return
	}

	userIDStr := userCtx.UserID.String()

	// Get staff profile
	staff, err := h.staffRepo.GetByUserID(userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_staff",
			"message": "User is not registered as staff",
		})
		return
	}

	// Get active trip ID from URL
	activeTripID := c.Param("id")
	if activeTripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_id",
			"message": "Active trip ID is required",
		})
		return
	}

	// Parse request
	var req UpdatePassengerCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Update passenger count
	err = h.activeTripService.UpdatePassengerCount(activeTripID, staff.ID, req.PassengerCount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "update_passengers_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Passenger count updated successfully",
		"passenger_count": req.PassengerCount,
	})
}

// GetActiveTripPassengers retrieves all passengers for an active scheduled trip
func (h *ActiveTripHandler) GetActiveTripPassengers(c *gin.Context) {
	scheduledTripIDStr := c.Query("scheduled_trip_id")
	if scheduledTripIDStr == "" {
		c.JSON(400, gin.H{"error": "scheduled_trip_id is required"})
		return
	}

	scheduledTripID, err := strconv.ParseUint(scheduledTripIDStr, 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid scheduled_trip_id"})
		return
	}

	passengers, err := h.activeTripService.GetActiveTripPassengers(uint(scheduledTripID))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if len(passengers) == 0 {
		c.JSON(200, gin.H{
			"status":  "success",
			"data":    []interface{}{},
			"message": "No passengers found for this trip",
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "success",
		"data":   passengers,
		"count":  len(passengers),
	})
}

// BoardPassenger updates a passenger's status to boarded
func (h *ActiveTripHandler) BoardPassenger(c *gin.Context) {
	var req struct {
		BookingID uint `json:"booking_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "booking_id is required"})
		return
	}

	err := h.activeTripService.BoardPassenger(req.BookingID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get updated booking details
	booking, err := h.activeTripService.GetBookingByID(req.BookingID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to retrieve updated booking"})
		return
	}

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Passenger boarded successfully",
		"data":    booking,
	})
}

// UpdatePassengerStatusToPending updates passenger status to waiting/pending
func (h *ActiveTripHandler) UpdatePassengerStatusToPending(c *gin.Context) {
	var req struct {
		BookingID uint `json:"booking_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "booking_id is required"})
		return
	}

	err := h.activeTripService.UpdatePassengerStatus(req.BookingID, "waiting")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Passenger status updated to waiting",
	})
}

// CancelPassenger cancels a passenger booking
func (h *ActiveTripHandler) CancelPassenger(c *gin.Context) {
	var req struct {
		BookingID uint `json:"booking_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "booking_id is required"})
		return
	}

	err := h.activeTripService.UpdatePassengerStatus(req.BookingID, "cancelled")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Passenger booking cancelled successfully",
	})
}

// VerifyPassengerByQR verifies a booking using QR code
func (h *ActiveTripHandler) VerifyPassengerByQR(c *gin.Context) {
	var req struct {
		Reference string `json:"reference" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "QR reference is required"})
		return
	}

	booking, err := h.activeTripService.VerifyBookingByQR(req.Reference)
	if err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": err.Error(),
			"valid":   false,
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Booking verified successfully",
		"valid":   true,
		"data":    booking,
	})
}

// ConfirmPassenger updates passenger status to confirmed
func (h *ActiveTripHandler) ConfirmPassenger(c *gin.Context) {
	var req struct {
		BookingID uint `json:"booking_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "booking_id is required"})
		return
	}

	err := h.activeTripService.UpdatePassengerStatus(req.BookingID, "confirmed")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Passenger booking confirmed",
	})
}

// MarkPassengerInTransit updates passenger status to in-transit
func (h *ActiveTripHandler) MarkPassengerInTransit(c *gin.Context) {
	var req struct {
		BookingID uint `json:"booking_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "booking_id is required"})
		return
	}

	err := h.activeTripService.UpdatePassengerStatus(req.BookingID, "in-transit")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Passenger marked as in-transit",
	})
}

// CompletePassenger updates passenger status to completed
func (h *ActiveTripHandler) CompletePassenger(c *gin.Context) {
	var req struct {
		BookingID uint `json:"booking_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "booking_id is required"})
		return
	}

	err := h.activeTripService.UpdatePassengerStatus(req.BookingID, "completed")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Passenger journey completed",
	})
}
