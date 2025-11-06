package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

type TripScheduleHandler struct {
	scheduleRepo     *database.TripScheduleRepository
	permitRepo       *database.RoutePermitRepository
	busOwnerRepo     *database.BusOwnerRepository
	busRepo          *database.BusRepository
	routeRepo        *database.BusOwnerRouteRepository
	tripGeneratorSvc *services.TripGeneratorService
}

func NewTripScheduleHandler(
	scheduleRepo *database.TripScheduleRepository,
	permitRepo *database.RoutePermitRepository,
	busOwnerRepo *database.BusOwnerRepository,
	busRepo *database.BusRepository,
	routeRepo *database.BusOwnerRouteRepository,
	tripGeneratorSvc *services.TripGeneratorService,
) *TripScheduleHandler {
	return &TripScheduleHandler{
		scheduleRepo:     scheduleRepo,
		permitRepo:       permitRepo,
		busOwnerRepo:     busOwnerRepo,
		busRepo:          busRepo,
		routeRepo:        routeRepo,
		tripGeneratorSvc: tripGeneratorSvc,
	}
}

// GetAllSchedules retrieves all trip schedules for the authenticated bus owner
// GET /api/v1/trip-schedules
func (h *TripScheduleHandler) GetAllSchedules(c *gin.Context) {
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

	schedules, err := h.scheduleRepo.GetByBusOwnerID(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedules"})
		return
	}

	c.JSON(http.StatusOK, schedules)
}

// GetSchedulesByPermit retrieves all trip schedules for a specific permit
// GET /api/v1/permits/:permitId/trip-schedules
func (h *TripScheduleHandler) GetSchedulesByPermit(c *gin.Context) {
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

	schedules, err := h.scheduleRepo.GetByPermitID(permitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedules"})
		return
	}

	c.JSON(http.StatusOK, schedules)
}

// GetScheduleByID retrieves a specific trip schedule by ID
// GET /api/v1/trip-schedules/:id
func (h *TripScheduleHandler) GetScheduleByID(c *gin.Context) {
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

	scheduleID := c.Param("id")

	schedule, err := h.scheduleRepo.GetByID(scheduleID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedule"})
		return
	}

	// Verify ownership
	if schedule.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, schedule)
}

// CreateSchedule creates a new trip schedule
// POST /api/v1/trip-schedules
func (h *TripScheduleHandler) CreateSchedule(c *gin.Context) {
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

	var req models.CreateTripScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify permit ownership
	permit, err := h.permitRepo.GetByID(req.PermitID)
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

	// Verify bus ownership if bus is specified
	if req.BusID != nil {
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

		// Verify bus is assigned to this permit
		if bus.PermitID != req.PermitID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Bus is not assigned to this permit"})
			return
		}
	}

	// Parse dates and times
	validFrom, _ := time.Parse("2006-01-02", req.ValidFrom)

	var validUntil *time.Time
	if req.ValidUntil != nil {
		parsed, _ := time.Parse("2006-01-02", *req.ValidUntil)
		validUntil = &parsed
	}

	// Parse specific dates if provided
	var specificDates models.DateArray
	if len(req.SpecificDates) > 0 {
		for _, dateStr := range req.SpecificDates {
			date, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format in specific_dates"})
				return
			}
			specificDates = append(specificDates, date)
		}
	}

	// Create schedule
	permitID := req.PermitID
	schedule := &models.TripSchedule{
		ID:                   uuid.New().String(),
		BusOwnerID:           busOwner.ID,
		PermitID:             &permitID,
		BusID:                req.BusID,
		ScheduleName:         req.ScheduleName,
		RecurrenceType:       models.RecurrenceType(req.RecurrenceType),
		RecurrenceDays:       models.IntArray(req.RecurrenceDays),
		SpecificDates:        specificDates,
		DepartureTime:        req.DepartureTime,
		EstimatedArrivalTime: req.EstimatedArrivalTime,
		Direction:            req.Direction,
		TripsPerDay:          req.TripsPerDay,
		BaseFare:             req.BaseFare,
		IsBookable:           req.IsBookable,
		MaxBookableSeats:     req.MaxBookableSeats,
		AdvanceBookingHours:  req.AdvanceBookingHours,
		DefaultDriverID:      req.DefaultDriverID,
		DefaultConductorID:   req.DefaultConductorID,
		SelectedStopIDs:      models.UUIDArray(req.SelectedStopIDs),
		IsActive:             true,
		ValidFrom:            validFrom,
		ValidUntil:           validUntil,
		Notes:                req.Notes,
	}

	// Default advance booking hours
	if schedule.AdvanceBookingHours == 0 {
		schedule.AdvanceBookingHours = 24
	}

	if err := h.scheduleRepo.Create(schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create schedule", "details": err.Error()})
		return
	}

	// IMMEDIATE GENERATION: Generate trips for next 14 days
	tripsGenerated, err := h.tripGeneratorSvc.GenerateTripsForNewSchedule(schedule)
	if err != nil {
		// Log error but don't fail the request - schedule was created successfully
		println("WARNING: Failed to generate trips for schedule:", schedule.ID, "Error:", err.Error())
	}

	// Return schedule with trip count
	response := gin.H{
		"schedule":        schedule,
		"trips_generated": tripsGenerated,
		"message":         "Schedule created successfully",
	}

	c.JSON(http.StatusCreated, response)
}

// UpdateSchedule updates an existing trip schedule
// PUT /api/v1/trip-schedules/:id
func (h *TripScheduleHandler) UpdateSchedule(c *gin.Context) {
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

	scheduleID := c.Param("id")

	// Get existing schedule
	schedule, err := h.scheduleRepo.GetByID(scheduleID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedule"})
		return
	}

	// Verify ownership
	if schedule.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req models.CreateTripScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	schedule.BusID = req.BusID
	schedule.ScheduleName = req.ScheduleName
	schedule.RecurrenceType = models.RecurrenceType(req.RecurrenceType)
	schedule.RecurrenceDays = models.IntArray(req.RecurrenceDays)
	schedule.DepartureTime = req.DepartureTime
	schedule.BaseFare = req.BaseFare
	schedule.IsBookable = req.IsBookable
	schedule.MaxBookableSeats = req.MaxBookableSeats
	schedule.AdvanceBookingHours = req.AdvanceBookingHours
	schedule.DefaultDriverID = req.DefaultDriverID
	schedule.DefaultConductorID = req.DefaultConductorID
	schedule.SelectedStopIDs = models.UUIDArray(req.SelectedStopIDs)
	schedule.Notes = req.Notes

	validFrom, _ := time.Parse("2006-01-02", req.ValidFrom)
	schedule.ValidFrom = validFrom

	if req.ValidUntil != nil {
		parsed, _ := time.Parse("2006-01-02", *req.ValidUntil)
		schedule.ValidUntil = &parsed
	} else {
		schedule.ValidUntil = nil
	}

	// Parse specific dates
	if len(req.SpecificDates) > 0 {
		var specificDates models.DateArray
		for _, dateStr := range req.SpecificDates {
			date, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format in specific_dates"})
				return
			}
			specificDates = append(specificDates, date)
		}
		schedule.SpecificDates = specificDates
	}

	if err := h.scheduleRepo.Update(schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update schedule", "details": err.Error()})
		return
	}

	// Regenerate future trips with updated schedule settings
	tripsGenerated, err := h.tripGeneratorSvc.RegenerateTripsForSchedule(schedule)
	if err != nil {
		println("WARNING: Failed to regenerate trips for schedule:", schedule.ID, "Error:", err.Error())
	}

	response := gin.H{
		"schedule":        schedule,
		"trips_generated": tripsGenerated,
		"message":         "Schedule updated successfully",
	}

	c.JSON(http.StatusOK, response)
}

// DeactivateSchedule deactivates a trip schedule
// POST /api/v1/trip-schedules/:id/deactivate
func (h *TripScheduleHandler) DeactivateSchedule(c *gin.Context) {
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

	scheduleID := c.Param("id")

	schedule, err := h.scheduleRepo.GetByID(scheduleID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedule"})
		return
	}

	// Verify ownership
	if schedule.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.scheduleRepo.Deactivate(scheduleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule deactivated successfully"})
}

// DeleteSchedule deletes a trip schedule
// DELETE /api/v1/trip-schedules/:id
func (h *TripScheduleHandler) DeleteSchedule(c *gin.Context) {
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

	scheduleID := c.Param("id")

	schedule, err := h.scheduleRepo.GetByID(scheduleID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedule"})
		return
	}

	// Verify ownership
	if schedule.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.scheduleRepo.Delete(scheduleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule deleted successfully"})
}

// CreateTimetable creates a new timetable (trip schedule) using the new timetable system
// POST /api/v1/timetables
func (h *TripScheduleHandler) CreateTimetable(c *gin.Context) {
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

	var req models.CreateTimetableRequest
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

	// Permit validation is optional - permit will be assigned later to specific trips
	// If provided, validate ownership only (no fare/seat limits enforcement)
	if req.PermitID != nil && *req.PermitID != "" {
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

		// Note: Permit details (approved_fare, approved_seating_capacity) are shown in UI
		// but not enforced here. User decides what fare and seats to set for the timetable.
	}

	// Create timetable
	schedule := &models.TripSchedule{
		ID:                   uuid.New().String(),
		BusOwnerID:           busOwner.ID,
		PermitID:             req.PermitID,
		CustomRouteID:        &req.CustomRouteID,
		ScheduleName:         req.ScheduleName,
		RecurrenceType:       models.RecurrenceType(req.RecurrenceType),
		RecurrenceDays:       req.RecurrenceDays,
		RecurrenceInterval:   req.RecurrenceInterval,
		DepartureTime:        req.DepartureTime,
		EstimatedArrivalTime: req.EstimatedArrivalTime,
		BaseFare:             req.BaseFare,
		IsBookable:           req.IsBookable,
		MaxBookableSeats:     &req.MaxBookableSeats,
		BookingAdvanceHours:  req.BookingAdvanceHours,
		IsActive:             true,
		Notes:                req.Notes,
	}

	if err := h.scheduleRepo.CreateTimetable(schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create timetable"})
		return
	}

	c.JSON(http.StatusCreated, schedule)
}
