package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type ScheduledTripHandler struct {
	tripRepo      *database.ScheduledTripRepository
	scheduleRepo  *database.TripScheduleRepository
	permitRepo    *database.RoutePermitRepository
	busOwnerRepo  *database.BusOwnerRepository
}

func NewScheduledTripHandler(
	tripRepo *database.ScheduledTripRepository,
	scheduleRepo *database.TripScheduleRepository,
	permitRepo *database.RoutePermitRepository,
	busOwnerRepo *database.BusOwnerRepository,
) *ScheduledTripHandler {
	return &ScheduledTripHandler{
		tripRepo:     tripRepo,
		scheduleRepo: scheduleRepo,
		permitRepo:   permitRepo,
		busOwnerRepo: busOwnerRepo,
	}
}

// GetTripsByDateRange retrieves scheduled trips within a date range
// GET /api/v1/scheduled-trips?start_date=2024-01-01&end_date=2024-01-31
func (h *ScheduledTripHandler) GetTripsByDateRange(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Parse query parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
		return
	}

	// Get all scheduled trips in date range (we'll filter by owner)
	allTrips, err := h.tripRepo.GetByDateRange(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trips"})
		return
	}

	// Filter trips that belong to this bus owner
	ownerTrips := []models.ScheduledTrip{}
	for _, trip := range allTrips {
		// Check if permit belongs to this bus owner
		permit, err := h.permitRepo.GetByID(trip.PermitID)
		if err == nil && permit.BusOwnerID == busOwner.ID {
			ownerTrips = append(ownerTrips, trip)
		}
	}

	c.JSON(http.StatusOK, ownerTrips)
}

// GetTripsByPermit retrieves scheduled trips for a specific permit
// GET /api/v1/permits/:permitId/scheduled-trips?start_date=2024-01-01&end_date=2024-01-31
func (h *ScheduledTripHandler) GetTripsByPermit(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	permitID := c.Param("permitId")

	// Verify permit ownership
	permit, err := h.permitRepo.GetByID(permitID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Permit not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permit"})
		return
	}

	if permit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Parse query parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
		return
	}

	trips, err := h.tripRepo.GetByPermitAndDateRange(permitID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trips"})
		return
	}

	c.JSON(http.StatusOK, trips)
}

// GetTripByID retrieves a specific scheduled trip by ID
// GET /api/v1/scheduled-trips/:id
func (h *ScheduledTripHandler) GetTripByID(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	tripID := c.Param("id")

	trip, err := h.tripRepo.GetByID(tripID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trip"})
		return
	}

	// Verify ownership through permit
	permit, err := h.permitRepo.GetByID(trip.PermitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}

	if permit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, trip)
}

// UpdateTrip updates a scheduled trip (staff assignment, status, etc.)
// PATCH /api/v1/scheduled-trips/:id
func (h *ScheduledTripHandler) UpdateTrip(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	tripID := c.Param("id")

	trip, err := h.tripRepo.GetByID(tripID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trip"})
		return
	}

	// Verify ownership
	permit, err := h.permitRepo.GetByID(trip.PermitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}

	if permit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req models.UpdateScheduledTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Update fields if provided
	if req.BusID != nil {
		trip.BusID = req.BusID
	}
	if req.AssignedDriverID != nil {
		trip.AssignedDriverID = req.AssignedDriverID
	}
	if req.AssignedConductorID != nil {
		trip.AssignedConductorID = req.AssignedConductorID
	}
	if req.Status != nil {
		trip.Status = models.ScheduledTripStatus(*req.Status)
	}
	if req.CancellationReason != nil {
		trip.CancellationReason = req.CancellationReason
	}

	if err := h.tripRepo.Update(trip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update trip", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, trip)
}

// CancelTrip cancels a scheduled trip
// POST /api/v1/scheduled-trips/:id/cancel
func (h *ScheduledTripHandler) CancelTrip(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	tripID := c.Param("id")

	trip, err := h.tripRepo.GetByID(tripID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trip"})
		return
	}

	// Verify ownership
	permit, err := h.permitRepo.GetByID(trip.PermitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}

	if permit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if trip can be cancelled
	if !trip.CanBeCancelled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip cannot be cancelled"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.tripRepo.Cancel(tripID, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel trip"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trip cancelled successfully"})
}

// GetBookableTrips retrieves bookable trips (public endpoint for passengers)
// GET /api/v1/bookable-trips?start_date=2024-01-01&end_date=2024-01-31
func (h *ScheduledTripHandler) GetBookableTrips(c *gin.Context) {
	// Parse query parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
		return
	}

	trips, err := h.tripRepo.GetBookableTrips(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trips"})
		return
	}

	c.JSON(http.StatusOK, trips)
}
