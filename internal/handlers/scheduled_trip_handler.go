package handlers

import (
	"database/sql"
	"fmt"
	"log"
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
	staffRepo    *database.BusStaffRepository
	settingRepo  *database.SystemSettingRepository
}

func NewScheduledTripHandler(
	tripRepo *database.ScheduledTripRepository,
	scheduleRepo *database.TripScheduleRepository,
	permitRepo *database.RoutePermitRepository,
	busOwnerRepo *database.BusOwnerRepository,
	routeRepo *database.BusOwnerRouteRepository,
	busRepo *database.BusRepository,
	staffRepo *database.BusStaffRepository,
	settingRepo *database.SystemSettingRepository,
) *ScheduledTripHandler {
	return &ScheduledTripHandler{
		tripRepo:     tripRepo,
		scheduleRepo: scheduleRepo,
		permitRepo:   permitRepo,
		busOwnerRepo: busOwnerRepo,
		routeRepo:    routeRepo,
		busRepo:      busRepo,
		staffRepo:    staffRepo,
		settingRepo:  settingRepo,
	}
}

// GetTripsByDateRange retrieves scheduled trips within a date range
// GET /api/v1/scheduled-trips?start_date=2024-01-01&end_date=2024-01-31
func (h *ScheduledTripHandler) GetTripsByDateRange(c *gin.Context) {
	fmt.Println("========== SCHEDULED TRIPS FETCH START ==========")

	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		fmt.Println("❌ ERROR: No user context found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	fmt.Printf("✅ STEP 1: Got user context - user_id=%s\n", userCtx.UserID.String())

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("❌ ERROR: Bus owner profile not found for user_id=%s\n", userCtx.UserID.String())
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		fmt.Printf("❌ ERROR: Failed to fetch bus owner: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	fmt.Printf("✅ STEP 2: Got bus owner - bus_owner_id=%s\n", busOwner.ID)

	// Parse query parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		fmt.Println("❌ ERROR: Missing start_date or end_date")
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		fmt.Printf("❌ ERROR: Invalid start_date format: %s\n", startDateStr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		fmt.Printf("❌ ERROR: Invalid end_date format: %s\n", endDateStr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
		return
	}

	fmt.Printf("✅ STEP 3: Parsed date range - from %s to %s\n", startDateStr, endDateStr)

	// Get all trip schedules (timetables) for this bus owner
	fmt.Printf("🔍 STEP 4: Querying trip_schedules WHERE bus_owner_id=%s\n", busOwner.ID)
	ownerSchedules, err := h.scheduleRepo.GetByBusOwnerID(busOwner.ID)
	if err != nil {
		fmt.Printf("❌ ERROR: Failed to fetch schedules: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch schedules"})
		return
	}

	fmt.Printf("✅ STEP 4 RESULT: Found %d schedules in trip_schedules table\n", len(ownerSchedules))

	// Extract schedule IDs
	scheduleIDs := make([]string, len(ownerSchedules))
	for i, schedule := range ownerSchedules {
		scheduleIDs[i] = schedule.ID
		fmt.Printf("   - Schedule[%d]: id=%s, name=%v\n", i+1, schedule.ID, schedule.ScheduleName)
	}

	if len(scheduleIDs) == 0 {
		fmt.Println("⚠️  WARNING: No schedules found - returning empty trips array")
		c.JSON(http.StatusOK, []models.ScheduledTripWithRouteInfo{})
		return
	}

	fmt.Printf("✅ STEP 5: Extracted %d schedule IDs\n", len(scheduleIDs))

	// Get trips directly by schedule IDs and date range
	fmt.Printf("🔍 STEP 6: Querying scheduled_trips WHERE trip_schedule_id IN (%d IDs) AND date BETWEEN %s AND %s\n",
		len(scheduleIDs), startDateStr, endDateStr)
	ownerTrips, err := h.tripRepo.GetByScheduleIDsAndDateRangeWithRouteInfo(scheduleIDs, startDate, endDate)
	if err != nil {
		fmt.Printf("❌ ERROR: Failed to fetch trips: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trips"})
		return
	}

	fmt.Printf("✅ STEP 6 RESULT: Found %d trips in scheduled_trips table with route info\n", len(ownerTrips))
	if len(ownerTrips) > 0 {
		for i, trip := range ownerTrips {
			routeInfo := "no route"
			if trip.OriginCity != nil && trip.DestinationCity != nil {
				routeInfo = fmt.Sprintf("%s - %s", *trip.OriginCity, *trip.DestinationCity)
			}
			fmt.Printf("   - Trip[%d]: id=%s, datetime=%s, route=%s\n",
				i+1, trip.ID, trip.DepartureDatetime.Format("2006-01-02 15:04:05"), routeInfo)
		}
	}

	fmt.Println("========== SCHEDULED TRIPS FETCH END (SUCCESS) ==========")
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
	}

	// VALIDATION: If updating bus_owner_route_id, validate it matches master route and direction
	if req.BusOwnerRouteID != nil {
		if trip.TripScheduleID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot update route for trips without a schedule"})
			return
		}

		// Get schedule to find its default route
		schedule, err := h.scheduleRepo.GetByID(*trip.TripScheduleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get schedule"})
			return
		}

		if schedule.BusOwnerRouteID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Schedule has no route defined"})
			return
		}

		// Get schedule's route (baseline)
		scheduleRoute, err := h.routeRepo.GetByID(*schedule.BusOwnerRouteID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get schedule's route"})
			return
		}

		// Get new route being proposed
		newRoute, err := h.routeRepo.GetByID(*req.BusOwnerRouteID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "New route not found"})
			return
		}

		// RULE 1: Must have same master_route_id
		if scheduleRoute.MasterRouteID != newRoute.MasterRouteID {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Route override rejected: new route must use the same master route as the schedule",
				"details": fmt.Sprintf("Schedule uses master route %s, new route uses %s",
					scheduleRoute.MasterRouteID, newRoute.MasterRouteID),
			})
			return
		}

		// RULE 2: Must have same direction (UP/DOWN)
		if scheduleRoute.Direction != newRoute.Direction {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Route override rejected: new route must have the same direction as the schedule",
				"details": fmt.Sprintf("Schedule direction: %s, new route direction: %s",
					scheduleRoute.Direction, newRoute.Direction),
			})
			return
		}

		// RULE 3: If permit is assigned, verify route is valid for permit
		if trip.PermitID != nil {
			permit, err := h.permitRepo.GetByID(*trip.PermitID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permit"})
				return
			}

			if permit.MasterRouteID != newRoute.MasterRouteID {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Route override rejected: permit is for a different master route",
					"details": fmt.Sprintf("Permit is for route %s, new route uses %s", permit.MasterRouteID, newRoute.MasterRouteID),
				})
				return
			}
		}

		// Validation passed - update the route
		trip.BusOwnerRouteID = req.BusOwnerRouteID
	}

	// Update other fields if provided
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
		if permit.ApprovedSeatingCapacity != nil && req.MaxBookableSeats > *permit.ApprovedSeatingCapacity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Max bookable seats exceeds permit approved seating capacity",
				"details": map[string]interface{}{
					"requested_seats": req.MaxBookableSeats,
					"approved_seats":  *permit.ApprovedSeatingCapacity,
				},
			})
			return
		}
	} // Parse departure datetime
	departureDatetime, _ := time.Parse(time.RFC3339, req.DepartureDatetime) // Already validated in Validate()

	// If parsing as RFC3339 fails, try ISO 8601 formats
	if departureDatetime.IsZero() {
		formats := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"}
		for _, format := range formats {
			if dt, err := time.Parse(format, req.DepartureDatetime); err == nil {
				departureDatetime = dt
				break
			}
		}
	}

	// Get system settings
	assignmentDeadlineHours := h.settingRepo.GetIntValue("assignment_deadline_hours", 2)
	defaultBookingAdvanceHours := h.settingRepo.GetIntValue("booking_advance_hours_default", 72)

	// Determine booking advance hours
	bookingAdvanceHours := defaultBookingAdvanceHours
	if req.BookingAdvanceHours != nil {
		bookingAdvanceHours = *req.BookingAdvanceHours
	}

	// Calculate assignment deadline
	assignmentDeadline := departureDatetime.Add(-time.Duration(assignmentDeadlineHours) * time.Hour)

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

	// Calculate actual arrival datetime
	var actualArrivalDatetime *time.Time
	if req.EstimatedDurationMinutes != nil {
		arrivalTime := departureDatetime.Add(time.Duration(*req.EstimatedDurationMinutes) * time.Minute)
		actualArrivalDatetime = &arrivalTime
	}

	// Create special trip
	trip := &models.ScheduledTrip{
		TripScheduleID:        nil, // Special trip - no timetable
		BusOwnerRouteID:       &req.CustomRouteID,
		PermitID:              req.PermitID,
		BusID:                 req.BusID,
		DepartureDatetime:     departureDatetime,
		ActualArrivalDatetime: actualArrivalDatetime,
		AssignedDriverID:      req.AssignedDriverID,
		AssignedConductorID:   req.AssignedConductorID,
		IsBookable:            req.IsBookable,
		TotalSeats:            req.MaxBookableSeats,
		AvailableSeats:        req.MaxBookableSeats,
		BookedSeats:           0,
		BaseFare:              req.BaseFare,
		BookingAdvanceHours:   bookingAdvanceHours,
		AssignmentDeadline:    &assignmentDeadline,
		Status:                models.ScheduledTripStatusScheduled,
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

// AssignStaffAndPermit assigns driver, conductor, and/or permit to a scheduled trip
// PATCH /api/v1/scheduled-trips/:id/assign
func (h *ScheduledTripHandler) AssignStaffAndPermit(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only bus owners can assign staff and permits"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus owner"})
		return
	}

	tripID := c.Param("id")

	// Get the trip
	trip, err := h.tripRepo.GetByID(tripID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trip"})
		return
	}

	// Verify ownership through trip schedule OR bus owner route
	var schedule *models.TripSchedule

	log.Printf("[AssignStaffToTrip] Trip ID: %s, TripScheduleID: %v, BusOwnerRouteID: %v, BusOwnerID: %s",
		tripID, trip.TripScheduleID, trip.BusOwnerRouteID, busOwner.ID)

	if trip.TripScheduleID == nil && trip.BusOwnerRouteID == nil {
		log.Printf("[AssignStaffToTrip] ERROR: Trip has neither schedule nor route")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot determine trip ownership - no schedule or route linked"})
		return
	}

	// Verify ownership via schedule if present
	if trip.TripScheduleID != nil {
		log.Printf("[AssignStaffToTrip] Verifying ownership via schedule ID: %s", *trip.TripScheduleID)
		var err error
		schedule, err = h.scheduleRepo.GetByID(*trip.TripScheduleID)
		if err != nil {
			log.Printf("[AssignStaffToTrip] ERROR: Failed to get schedule: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership via schedule", "details": err.Error()})
			return
		}

		log.Printf("[AssignStaffToTrip] Schedule found. Schedule BusOwnerID: %s, Current BusOwnerID: %s",
			schedule.BusOwnerID, busOwner.ID)

		if schedule.BusOwnerID != busOwner.ID {
			log.Printf("[AssignStaffToTrip] ERROR: Ownership mismatch - access denied")
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		log.Printf("[AssignStaffToTrip] Ownership verified via schedule ✓")
	}

	// Verify ownership via bus owner route if present (and no schedule check done)
	if trip.TripScheduleID == nil && trip.BusOwnerRouteID != nil {
		log.Printf("[AssignStaffToTrip] Verifying ownership via route ID: %s", *trip.BusOwnerRouteID)
		route, err := h.routeRepo.GetByID(*trip.BusOwnerRouteID)
		if err != nil {
			log.Printf("[AssignStaffToTrip] ERROR: Failed to get route: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership via route", "details": err.Error()})
			return
		}

		log.Printf("[AssignStaffToTrip] Route found. Route BusOwnerID: %s, Current BusOwnerID: %s",
			route.BusOwnerID, busOwner.ID)

		if route.BusOwnerID != busOwner.ID {
			log.Printf("[AssignStaffToTrip] ERROR: Ownership mismatch - access denied")
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		log.Printf("[AssignStaffToTrip] Ownership verified via route ✓")
	}

	// Parse request
	var req struct {
		DriverID    *string `json:"driver_id"`
		ConductorID *string `json:"conductor_id"`
		PermitID    *string `json:"permit_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Validate at least one field is provided
	if req.DriverID == nil && req.ConductorID == nil && req.PermitID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one of driver_id, conductor_id, or permit_id must be provided"})
		return
	}

	// Validate driver if provided
	if req.DriverID != nil && *req.DriverID != "" {
		staff, err := h.staffRepo.GetByID(*req.DriverID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Driver not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate driver"})
			return
		}

		// Verify driver belongs to this bus owner
		if staff.BusOwnerID == nil || *staff.BusOwnerID != busOwner.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Driver does not belong to your organization"})
			return
		}

		// Verify driver type
		if staff.StaffType != "driver" && staff.StaffType != "both" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Selected staff is not a driver"})
			return
		}

		// Verify employment status
		if staff.EmploymentStatus != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Driver is not actively employed"})
			return
		}

		// Check if license is expired (compare with trip departure date)
		tripDate := time.Date(trip.DepartureDatetime.Year(), trip.DepartureDatetime.Month(), trip.DepartureDatetime.Day(), 0, 0, 0, 0, time.UTC)
		if staff.LicenseExpiryDate != nil && staff.LicenseExpiryDate.Before(tripDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Driver's license will be expired on trip date"})
			return
		}
	}

	// Validate conductor if provided
	if req.ConductorID != nil && *req.ConductorID != "" {
		staff, err := h.staffRepo.GetByID(*req.ConductorID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Conductor not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate conductor"})
			return
		}

		// Verify conductor belongs to this bus owner
		if staff.BusOwnerID == nil || *staff.BusOwnerID != busOwner.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Conductor does not belong to your organization"})
			return
		}

		// Verify conductor type
		if staff.StaffType != "conductor" && staff.StaffType != "both" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Selected staff is not a conductor"})
			return
		}

		// Verify employment status
		if staff.EmploymentStatus != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Conductor is not actively employed"})
			return
		}
	}

	// Validate permit if provided
	if req.PermitID != nil && *req.PermitID != "" {
		permit, err := h.permitRepo.GetByID(*req.PermitID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Permit not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate permit"})
			return
		}

		// Verify permit belongs to this bus owner
		if permit.BusOwnerID != busOwner.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Permit does not belong to your organization"})
			return
		}

		// Verify permit is verified (approved by authorities)
		log.Printf("[AssignStaffToTrip] Permit status check - Permit ID: %s, Status: %s", req.PermitID, permit.Status)
		if permit.Status != models.VerificationVerified {
			log.Printf("[AssignStaffToTrip] ❌ Permit status is '%s', expected 'verified'", permit.Status)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Permit must be verified before assignment",
				"details": fmt.Sprintf("Current permit status: %s", permit.Status),
			})
			return
		}
		log.Printf("[AssignStaffToTrip] ✓ Permit is verified")

		// Verify permit is valid on trip date
		// Extract date from DepartureDatetime for comparison with permit dates
		tripDate := time.Date(trip.DepartureDatetime.Year(), trip.DepartureDatetime.Month(), trip.DepartureDatetime.Day(), 0, 0, 0, 0, time.UTC)
		if permit.IssueDate.After(tripDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Permit is not yet valid on trip date"})
			return
		}
		if permit.ExpiryDate.Before(tripDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Permit will be expired on trip date"})
			return
		}

		// Verify permit matches the route
		// Get the effective route for the trip (trip override or schedule's route)
		var routeID string
		if trip.BusOwnerRouteID != nil {
			routeID = *trip.BusOwnerRouteID
		} else if schedule != nil && schedule.BusOwnerRouteID != nil {
			routeID = *schedule.BusOwnerRouteID
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trip has no route assigned"})
			return
		}

		route, err := h.routeRepo.GetByID(routeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch route"})
			return
		}

		// Verify permit covers this route
		if permit.MasterRouteID != route.MasterRouteID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Permit does not cover this route"})
			return
		}
	}

	// Perform the assignment
	err = h.tripRepo.AssignStaffAndPermit(tripID, req.DriverID, req.ConductorID, req.PermitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign staff and permit", "details": err.Error()})
		return
	}

	// Fetch updated trip
	updatedTrip, err := h.tripRepo.GetByID(tripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated trip"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff and permit assigned successfully",
		"trip":    updatedTrip,
	})
}
