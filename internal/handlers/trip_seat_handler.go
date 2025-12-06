package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// TripSeatHandler handles trip seats and manual bookings API endpoints
type TripSeatHandler struct {
	tripSeatRepo      *database.TripSeatRepository
	manualBookingRepo *database.ManualBookingRepository
	tripRepo          *database.ScheduledTripRepository
	busOwnerRepo      *database.BusOwnerRepository
	routeRepo         *database.BusOwnerRouteRepository
}

// NewTripSeatHandler creates a new TripSeatHandler
func NewTripSeatHandler(
	tripSeatRepo *database.TripSeatRepository,
	manualBookingRepo *database.ManualBookingRepository,
	tripRepo *database.ScheduledTripRepository,
	busOwnerRepo *database.BusOwnerRepository,
	routeRepo *database.BusOwnerRouteRepository,
) *TripSeatHandler {
	return &TripSeatHandler{
		tripSeatRepo:      tripSeatRepo,
		manualBookingRepo: manualBookingRepo,
		tripRepo:          tripRepo,
		busOwnerRepo:      busOwnerRepo,
		routeRepo:         routeRepo,
	}
}

// ===========================================================================
// TRIP SEATS ENDPOINTS
// ===========================================================================

// GetTripSeats returns all seats for a scheduled trip
// GET /api/v1/trips/:tripId/seats
func (h *TripSeatHandler) GetTripSeats(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Get seats with booking info
	seats, err := h.tripSeatRepo.GetByScheduledTripIDWithBookingInfo(tripID)
	if err != nil {
		fmt.Printf("Error getting trip seats: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trip seats"})
		return
	}

	// Get summary
	summary, err := h.tripSeatRepo.GetSummary(tripID)
	if err != nil {
		fmt.Printf("Error getting seat summary: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"seats":   seats,
		"summary": summary,
	})
}

// GetTripSeatSummary returns seat availability summary for a trip
// GET /api/v1/trips/:tripId/seats/summary
func (h *TripSeatHandler) GetTripSeatSummary(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	summary, err := h.tripSeatRepo.GetSummary(tripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get seat summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// CreateTripSeats creates trip seats from a seat layout template
// POST /api/v1/trips/:tripId/seats/create
func (h *TripSeatHandler) CreateTripSeats(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	var req models.CreateTripSeatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Override tripId from URL
	req.ScheduledTripID = tripID

	// Verify the trip exists and belongs to this bus owner
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// TODO: Add ownership verification here

	// Create trip seats from layout
	count, err := h.tripSeatRepo.CreateTripSeatsFromLayout(req.ScheduledTripID, req.SeatLayoutID, req.BaseFare)
	if err != nil {
		fmt.Printf("Error creating trip seats: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trip seats: " + err.Error()})
		return
	}

	fmt.Printf("Created %d trip seats for trip %s by user %s\n", count, tripID, userCtx.UserID)

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Trip seats created successfully",
		"seats_count": count,
	})
}

// BlockSeats blocks one or more seats
// POST /api/v1/trips/:tripId/seats/block
func (h *TripSeatHandler) BlockSeats(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.BlockSeatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify seats belong to this trip
	seats, err := h.tripSeatRepo.GetByIDs(req.SeatIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify seats"})
		return
	}

	for _, seat := range seats {
		if seat.ScheduledTripID != tripID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Seat " + seat.SeatNumber + " does not belong to this trip"})
			return
		}
	}

	// Block the seats
	count, err := h.tripSeatRepo.BlockSeats(req.SeatIDs, userCtx.UserID.String(), req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to block seats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Seats blocked successfully",
		"blocked_count": count,
	})
}

// UnblockSeats unblocks one or more seats
// POST /api/v1/trips/:tripId/seats/unblock
func (h *TripSeatHandler) UnblockSeats(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	var req models.UnblockSeatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify seats belong to this trip
	seats, err := h.tripSeatRepo.GetByIDs(req.SeatIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify seats"})
		return
	}

	for _, seat := range seats {
		if seat.ScheduledTripID != tripID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Seat " + seat.SeatNumber + " does not belong to this trip"})
			return
		}
	}

	// Unblock the seats
	count, err := h.tripSeatRepo.UnblockSeats(req.SeatIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unblock seats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Seats unblocked successfully",
		"unblocked_count": count,
	})
}

// UpdateSeatPrices updates prices for one or more seats
// PUT /api/v1/trips/:tripId/seats/price
func (h *TripSeatHandler) UpdateSeatPrices(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	var req models.UpdateSeatPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify seats belong to this trip
	seats, err := h.tripSeatRepo.GetByIDs(req.SeatIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify seats"})
		return
	}

	for _, seat := range seats {
		if seat.ScheduledTripID != tripID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Seat " + seat.SeatNumber + " does not belong to this trip"})
			return
		}
	}

	// Update prices
	count, err := h.tripSeatRepo.UpdateSeatPrices(req.SeatIDs, req.NewPrice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update seat prices"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Seat prices updated successfully",
		"updated_count": count,
	})
}

// ===========================================================================
// MANUAL BOOKINGS ENDPOINTS
// ===========================================================================

// CreateManualBooking creates a phone/agent/walk-in booking
// POST /api/v1/trips/:tripId/manual-bookings
func (h *TripSeatHandler) CreateManualBooking(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateManualBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Override tripId from URL
	req.ScheduledTripID = tripID

	// Verify seats belong to this trip and are available
	seats, err := h.tripSeatRepo.GetByIDs(req.SeatIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify seats"})
		return
	}

	if len(seats) != len(req.SeatIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Some seats not found"})
		return
	}

	for _, seat := range seats {
		if seat.ScheduledTripID != tripID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Seat " + seat.SeatNumber + " does not belong to this trip"})
			return
		}
		if seat.Status != models.TripSeatStatusAvailable {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Seat " + seat.SeatNumber + " is not available"})
			return
		}
	}

	// Get trip info for route name and departure time
	trip, err := h.tripRepo.GetByID(tripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trip info"})
		return
	}

	// Get route info
	var routeName string
	if trip.BusOwnerRouteID != nil {
		route, err := h.routeRepo.GetByID(*trip.BusOwnerRouteID)
		if err == nil {
			routeName = route.CustomRouteName
		}
	}
	if routeName == "" {
		routeName = "Unknown Route"
	}

	// Create the booking
	booking := &models.ManualSeatBooking{
		ScheduledTripID:   tripID,
		CreatedByUserID:   userCtx.UserID.String(),
		BookingType:       models.ManualBookingType(req.BookingType),
		PassengerName:     req.PassengerName,
		PassengerPhone:    req.PassengerPhone,
		PassengerNIC:      req.PassengerNIC,
		PassengerNotes:    req.PassengerNotes,
		RouteName:         routeName,
		BoardingStopID:    req.BoardingStopID,
		BoardingStopName:  req.BoardingStopName,
		AlightingStopID:   req.AlightingStopID,
		AlightingStopName: req.AlightingStopName,
		DepartureDatetime: trip.DepartureDatetime,
		PaymentStatus:     models.ManualBookingPaymentStatus(req.PaymentStatus),
		AmountPaid:        req.AmountPaid,
		PaymentMethod:     req.PaymentMethod,
		PaymentNotes:      req.PaymentNotes,
	}

	result, err := h.manualBookingRepo.Create(booking, req.SeatIDs, h.tripSeatRepo)
	if err != nil {
		fmt.Printf("Error creating manual booking: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create booking: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetManualBookings returns all manual bookings for a trip
// GET /api/v1/trips/:tripId/manual-bookings
func (h *TripSeatHandler) GetManualBookings(c *gin.Context) {
	tripID := c.Param("tripId")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	bookings, err := h.manualBookingRepo.GetByScheduledTripID(tripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get manual bookings"})
		return
	}

	// Get seats for each booking
	var results []models.ManualBookingWithSeats
	for _, booking := range bookings {
		seats, _ := h.manualBookingRepo.GetBookingSeats(booking.ID)
		results = append(results, models.ManualBookingWithSeats{
			ManualSeatBooking: booking,
			Seats:             seats,
		})
	}

	c.JSON(http.StatusOK, results)
}

// GetManualBooking returns a single manual booking
// GET /api/v1/manual-bookings/:bookingId
func (h *TripSeatHandler) GetManualBooking(c *gin.Context) {
	bookingID := c.Param("bookingId")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Booking ID is required"})
		return
	}

	result, err := h.manualBookingRepo.GetWithSeats(bookingID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get booking"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetManualBookingByReference returns a booking by its reference number
// GET /api/v1/manual-bookings/reference/:ref
func (h *TripSeatHandler) GetManualBookingByReference(c *gin.Context) {
	ref := c.Param("ref")
	if ref == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Booking reference is required"})
		return
	}

	booking, err := h.manualBookingRepo.GetByBookingReference(ref)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get booking"})
		return
	}

	seats, _ := h.manualBookingRepo.GetBookingSeats(booking.ID)

	c.JSON(http.StatusOK, models.ManualBookingWithSeats{
		ManualSeatBooking: *booking,
		Seats:             seats,
	})
}

// UpdateManualBookingPayment updates payment information for a booking
// PUT /api/v1/manual-bookings/:bookingId/payment
func (h *TripSeatHandler) UpdateManualBookingPayment(c *gin.Context) {
	bookingID := c.Param("bookingId")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Booking ID is required"})
		return
	}

	var req models.UpdateManualBookingPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.manualBookingRepo.UpdatePayment(
		bookingID,
		models.ManualBookingPaymentStatus(req.PaymentStatus),
		req.AmountPaid,
		req.PaymentMethod,
		req.PaymentNotes,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment updated successfully"})
}

// CancelManualBooking cancels a manual booking and releases the seats
// DELETE /api/v1/manual-bookings/:bookingId
func (h *TripSeatHandler) CancelManualBooking(c *gin.Context) {
	bookingID := c.Param("bookingId")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Booking ID is required"})
		return
	}

	var req models.CancelManualBookingRequest
	c.ShouldBindJSON(&req) // Optional body

	err := h.manualBookingRepo.Cancel(bookingID, req.Reason, h.tripSeatRepo)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found or already cancelled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel booking"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Booking cancelled successfully"})
}

// UpdateManualBookingStatus updates the status of a manual booking
// PUT /api/v1/manual-bookings/:bookingId/status
func (h *TripSeatHandler) UpdateManualBookingStatus(c *gin.Context) {
	bookingID := c.Param("bookingId")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Booking ID is required"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required,oneof=confirmed checked_in boarded completed no_show"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.manualBookingRepo.UpdateStatus(bookingID, models.ManualBookingStatus(req.Status))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status updated successfully"})
}

// SearchManualBookingsByPhone searches bookings by passenger phone
// GET /api/v1/manual-bookings/search?phone=077...
func (h *TripSeatHandler) SearchManualBookingsByPhone(c *gin.Context) {
	phone := c.Query("phone")
	if phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phone number is required"})
		return
	}

	bookings, err := h.manualBookingRepo.SearchByPassengerPhone(phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search bookings"})
		return
	}

	c.JSON(http.StatusOK, bookings)
}
