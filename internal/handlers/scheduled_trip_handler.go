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
	tripRepo     *database.ScheduledTripRepository
	scheduleRepo *database.TripScheduleRepository
	permitRepo   *database.RoutePermitRepository
	busOwnerRepo *database.BusOwnerRepository
	routeRepo    *database.BusOwnerRouteRepository
	busRepo      *database.BusRepository
	settingRepo  *database.SystemSettingRepository
}

func NewScheduledTripHandler(
	tripRepo *database.ScheduledTripRepository,
	scheduleRepo *database.TripScheduleRepository,
	permitRepo *database.RoutePermitRepository,
	busOwnerRepo *database.BusOwnerRepository,
	routeRepo *database.BusOwnerRouteRepository,
	busRepo *database.BusRepository,
	settingRepo *database.SystemSettingRepository,
) *ScheduledTripHandler {
	return &ScheduledTripHandler{
		tripRepo:     tripRepo,
		scheduleRepo: scheduleRepo,
		permitRepo:   permitRepo,
		busOwnerRepo: busOwnerRepo,
		routeRepo:    routeRepo,
		busRepo:      busRepo,
		settingRepo:  settingRepo,
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

	// Get all trip schedules (timetables) for this bus owner
	ownerSchedules, err := h.scheduleRepo.GetByBusOwnerID(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedules"})
		return
	}

	// Extract schedule IDs
	scheduleIDs := make([]string, len(ownerSchedules))
	for i, schedule := range ownerSchedules {
		scheduleIDs[i] = schedule.ID
	}

	// Get trips directly by schedule IDs and date range
	ownerTrips, err := h.tripRepo.GetByScheduleIDsAndDateRange(scheduleIDs, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trips"})
		return
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
if trip.PermitID == nil {
	c.JSON(http.StatusForbidden, gin.H{"error": "Trip has no permit assigned"})
	return
}
permit, err := h.permitRepo.GetByID(*trip.PermitID)
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
if trip.PermitID == nil {
	c.JSON(http.StatusForbidden, gin.H{"error": "Trip has no permit assigned"})
	return
}
permit, err := h.permitRepo.GetByID(*trip.PermitID)
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
}	// Update fields if provided
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
if trip.PermitID == nil {
	c.JSON(http.StatusForbidden, gin.H{"error": "Trip has no permit assigned"})
	return
}
permit, err := h.permitRepo.GetByID(*trip.PermitID)
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

// CreateSpecialTrip creates a special one-time trip (not from timetable)
// POST /api/v1/special-trips
func (h *ScheduledTripHandler) CreateSpecialTrip(c *gin.Context) {
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

	var req models.CreateSpecialTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify custom route ownership
	customRoute, err := h.routeRepo.GetByID(req.CustomRouteID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Custom route not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch custom route"})
		return
	}

	if customRoute.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this custom route"})
		return
	}

	// Verify permit ownership (optional)
	if req.PermitID != nil {
		permit, err := h.permitRepo.GetByID(*req.PermitID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Permit not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permit"})
		return
	}

	if permit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this permit"})
		return
	}

	// Check permit is valid
	if !permit.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Permit is not valid or expired"})
		return
	}

	// Validate fare against permit approved fare
	if req.BaseFare > permit.ApprovedFare {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Base fare exceeds permit approved fare",
			"details": map[string]interface{}{
				"requested_fare": req.BaseFare,
				"approved_fare":  permit.ApprovedFare,
			},
		})
		return
	}

	// Validate max bookable seats against permit approved seating capacity
	if req.MaxBookableSeats > permit.ApprovedSeatingCapacity {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Max bookable seats exceeds permit approved seating capacity",
			"details": map[string]interface{}{
			"requested_seats": req.MaxBookableSeats,
			"approved_seats":  permit.ApprovedSeatingCapacity,
		},
	})
	return
	}
}

// Parse trip date
tripDate, _ := time.Parse("2006-01-02", req.TripDate) // Already validated in Validate()	// Parse departure time
	var departureHour, departureMinute int
	if t, err := time.Parse("15:04", req.DepartureTime); err == nil {
		departureHour = t.Hour()
		departureMinute = t.Minute()
	} else if t, err := time.Parse("15:04:05", req.DepartureTime); err == nil {
		departureHour = t.Hour()
		departureMinute = t.Minute()
	}

	// Calculate departure datetime
	departureDateTime := time.Date(
		tripDate.Year(), tripDate.Month(), tripDate.Day(),
		departureHour, departureMinute, 0, 0, time.Local,
	)

	// Get system settings
	assignmentDeadlineHours := h.settingRepo.GetIntValue("assignment_deadline_hours", 2)
	defaultBookingAdvanceHours := h.settingRepo.GetIntValue("booking_advance_hours_default", 72)

	// Determine booking advance hours
	bookingAdvanceHours := defaultBookingAdvanceHours
	if req.BookingAdvanceHours != nil {
		bookingAdvanceHours = *req.BookingAdvanceHours
	}

	// Calculate assignment deadline
	assignmentDeadline := departureDateTime.Add(-time.Duration(assignmentDeadlineHours) * time.Hour)

	// Check if trip is too soon (assignment deadline has passed or will pass soon)
	now := time.Now()
	requiresImmediateAssignment := assignmentDeadline.Before(now) || assignmentDeadline.Sub(now) < 1*time.Hour

	if requiresImmediateAssignment {
		// Verify resources are assigned
		if req.BusID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Trip date is too soon. Bus assignment is required.",
				"details": map[string]interface{}{
					"assignment_deadline": assignmentDeadline.Format(time.RFC3339),
					"current_time":        now.Format(time.RFC3339),
				},
			})
			return
		}

		// Verify bus ownership
		bus, err := h.busRepo.GetByID(*req.BusID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Bus not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus"})
			return
		}

		if bus.BusOwnerID != busOwner.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this bus"})
			return
		}
	}

	// Create special trip
	trip := &models.ScheduledTrip{
		TripScheduleID:       nil, // Special trip - no timetable
		CustomRouteID:        &req.CustomRouteID,
		PermitID:             req.PermitID,
		BusID:                req.BusID,
		TripDate:             tripDate,
		DepartureTime:        req.DepartureTime,
		EstimatedArrivalTime: req.EstimatedArrivalTime,
		AssignedDriverID:     req.AssignedDriverID,
		AssignedConductorID:  req.AssignedConductorID,
		IsBookable:           req.IsBookable,
		TotalSeats:           req.MaxBookableSeats,
		AvailableSeats:       req.MaxBookableSeats,
		BookedSeats:          0,
		BaseFare:             req.BaseFare,
		BookingAdvanceHours:  bookingAdvanceHours,
		AssignmentDeadline:   &assignmentDeadline,
		Status:               models.ScheduledTripStatusScheduled,
	}

	if err := h.tripRepo.Create(trip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create special trip"})
		return
	}

	c.JSON(http.StatusCreated, trip)
}

// PublishTrip publishes a single scheduled trip
// PUT /api/v1/scheduled-trips/:id/publish
func (h *ScheduledTripHandler) PublishTrip(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tripID := c.Param("id")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Get bus owner
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only bus owners can publish trips"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus owner"})
		return
	}

	// Publish the trip
	if err := h.tripRepo.PublishTrip(tripID, busOwner.ID); err != nil {
		if err.Error() == "trip not found or unauthorized" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish trip"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trip published successfully",
		"trip_id": tripID,
	})
}

// UnpublishTrip unpublishes a single scheduled trip
// PUT /api/v1/scheduled-trips/:id/unpublish
func (h *ScheduledTripHandler) UnpublishTrip(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tripID := c.Param("id")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Get bus owner
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only bus owners can unpublish trips"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus owner"})
		return
	}

	// Unpublish the trip
	if err := h.tripRepo.UnpublishTrip(tripID, busOwner.ID); err != nil {
		if err.Error() == "trip not found or unauthorized" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpublish trip"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trip unpublished successfully",
		"trip_id": tripID,
	})
}

// BulkPublishTrips publishes multiple scheduled trips at once
// POST /api/v1/scheduled-trips/bulk-publish
func (h *ScheduledTripHandler) BulkPublishTrips(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		TripIDs []string `json:"trip_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if len(req.TripIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one trip ID is required"})
		return
	}

	// Get bus owner
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only bus owners can publish trips"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus owner"})
		return
	}

	// Bulk publish trips
	publishedCount, err := h.tripRepo.BulkPublishTrips(req.TripIDs, busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish trips"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Trips published successfully",
		"published_count": publishedCount,
		"requested_count": len(req.TripIDs),
	})
}

// BulkUnpublishTrips unpublishes multiple scheduled trips at once
// POST /api/v1/scheduled-trips/bulk-unpublish
func (h *ScheduledTripHandler) BulkUnpublishTrips(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		TripIDs []string `json:"trip_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if len(req.TripIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one trip ID is required"})
		return
	}

	// Get bus owner
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only bus owners can unpublish trips"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus owner"})
		return
	}

	// Bulk unpublish trips
	unpublishedCount, err := h.tripRepo.BulkUnpublishTrips(req.TripIDs, busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpublish trips"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Trips unpublished successfully",
		"unpublished_count": unpublishedCount,
		"requested_count":   len(req.TripIDs),
	})
}
