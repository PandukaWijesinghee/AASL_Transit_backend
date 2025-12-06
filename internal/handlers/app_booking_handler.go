package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// AppBookingHandler handles passenger app booking operations
type AppBookingHandler struct {
	bookingRepo  *database.AppBookingRepository
	tripRepo     *database.ScheduledTripRepository
	tripSeatRepo *database.TripSeatRepository
	routeRepo    *database.BusOwnerRouteRepository
}

// NewAppBookingHandler creates a new AppBookingHandler
func NewAppBookingHandler(
	bookingRepo *database.AppBookingRepository,
	tripRepo *database.ScheduledTripRepository,
	tripSeatRepo *database.TripSeatRepository,
	routeRepo *database.BusOwnerRouteRepository,
) *AppBookingHandler {
	return &AppBookingHandler{
		bookingRepo:  bookingRepo,
		tripRepo:     tripRepo,
		tripSeatRepo: tripSeatRepo,
		routeRepo:    routeRepo,
	}
}

// CreateBooking creates a new bus booking
// @Summary Create a new bus booking
// @Description Create a new bus booking with seat selection (passenger app)
// @Tags App Bookings
// @Accept json
// @Produce json
// @Param request body models.CreateAppBookingRequest true "Booking request"
// @Success 201 {object} models.BookingResponse "Booking created successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 409 {object} map[string]interface{} "Seats not available"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/bookings [post]
func (h *AppBookingHandler) CreateBooking(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateAppBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get trip details
	trip, err := h.tripRepo.GetByID(req.ScheduledTripID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trip details"})
		return
	}

	// Check trip is bookable
	if !trip.IsBookable {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This trip is not available for booking"})
		return
	}

	// Check trip is not in the past
	if trip.DepartureDatetime.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot book a trip that has already departed"})
		return
	}

	// Check seat availability
	tripSeatIDs := make([]string, len(req.Seats))
	for i, seat := range req.Seats {
		tripSeatIDs[i] = seat.TripSeatID
	}

	availableSeats, err := h.bookingRepo.CheckSeatAvailability(tripSeatIDs)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if len(availableSeats) != len(req.Seats) {
		c.JSON(http.StatusConflict, gin.H{"error": "Some seats are no longer available"})
		return
	}

	// Build seat price map
	seatPriceMap := make(map[string]float64)
	for _, seat := range availableSeats {
		seatPriceMap[seat.ID] = seat.SeatPrice
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

	// Calculate totals
	var totalFare float64
	for i := range req.Seats {
		price := seatPriceMap[req.Seats[i].TripSeatID]
		if price == 0 {
			price = trip.BaseFare // Fallback to base fare
		}
		req.Seats[i].SeatPrice = price
		totalFare += price
	}

	// Build master booking
	booking := &models.MasterBooking{
		UserID:         userCtx.UserID.String(),
		BookingType:    models.BookingTypeBusOnly,
		BusTotal:       totalFare,
		Subtotal:       totalFare,
		TotalAmount:    totalFare,
		PaymentStatus:  models.MasterPaymentPending,
		BookingStatus:  models.MasterBookingPending,
		PassengerName:  req.PassengerName,
		PassengerPhone: req.PassengerPhone,
		PassengerEmail: req.PassengerEmail,
		BookingSource:  models.BookingSourceApp,
		DeviceInfo:     req.DeviceInfo,
	}

	// Build bus booking (normalized - only store IDs, not denormalized data)
	busBooking := &models.BusBooking{
		ScheduledTripID: req.ScheduledTripID,
		BoardingStopID:  req.BoardingStopID,
		AlightingStopID: req.AlightingStopID,
		NumberOfSeats:   len(req.Seats),
		FarePerSeat:     trip.BaseFare,
		TotalFare:       totalFare,
		Status:          models.BusBookingPending,
		SpecialRequests: req.SpecialRequests,
		// Denormalized fields for response only (not stored in DB)
		RouteName:         routeName,
		BoardingStopName:  req.BoardingStopName,
		AlightingStopName: req.AlightingStopName,
		DepartureDatetime: &trip.DepartureDatetime,
	}

	// Build seat records (normalized - seat info from trip_seats)
	seats := make([]models.BusBookingSeat, len(req.Seats))
	for i, seatReq := range req.Seats {
		seats[i] = models.BusBookingSeat{
			TripSeatID:         &seatReq.TripSeatID,
			PassengerName:      seatReq.PassengerName,
			PassengerPhone:     seatReq.PassengerPhone,
			PassengerEmail:     seatReq.PassengerEmail,
			PassengerGender:    seatReq.PassengerGender,
			PassengerNIC:       seatReq.PassengerNIC,
			IsPrimaryPassenger: seatReq.IsPrimary,
			Status:             models.SeatBookingPending,
			// Denormalized fields for response only (not stored in DB)
			SeatNumber: seatReq.SeatNumber,
			SeatType:   seatReq.SeatType,
			SeatPrice:  seatReq.SeatPrice,
		}
	}

	// Create booking
	response, err := h.bookingRepo.CreateBooking(booking, busBooking, seats, h.tripSeatRepo)
	if err != nil {
		fmt.Printf("Error creating booking: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create booking", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// GetMyBookings retrieves bookings for the authenticated user
// @Summary Get my bookings
// @Description Get all bookings for the authenticated passenger
// @Tags App Bookings
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {array} models.BookingListItem "List of bookings"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/bookings [get]
func (h *AppBookingHandler) GetMyBookings(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	bookings, err := h.bookingRepo.GetBookingsByUserID(userCtx.UserID.String(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bookings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookings": bookings,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetUpcomingBookings retrieves upcoming bookings for the authenticated user
// @Summary Get upcoming bookings
// @Description Get upcoming (not departed) bookings for the authenticated passenger
// @Tags App Bookings
// @Produce json
// @Success 200 {array} models.BookingListItem "List of upcoming bookings"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/bookings/upcoming [get]
func (h *AppBookingHandler) GetUpcomingBookings(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	bookings, err := h.bookingRepo.GetUpcomingBookingsByUserID(userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bookings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bookings": bookings})
}

// GetBookingByID retrieves a specific booking
// @Summary Get booking by ID
// @Description Get booking details by ID
// @Tags App Bookings
// @Produce json
// @Param id path string true "Booking ID"
// @Success 200 {object} models.MasterBooking "Booking details"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "Not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/bookings/{id} [get]
func (h *AppBookingHandler) GetBookingByID(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	bookingID := c.Param("id")
	booking, err := h.bookingRepo.GetBookingByID(bookingID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get booking"})
		return
	}

	// Check ownership
	if booking.UserID != userCtx.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to view this booking"})
		return
	}

	c.JSON(http.StatusOK, booking)
}

// GetBookingByReference retrieves a booking by reference
// @Summary Get booking by reference
// @Description Get booking details by booking reference
// @Tags App Bookings
// @Produce json
// @Param reference path string true "Booking Reference"
// @Success 200 {object} models.MasterBooking "Booking details"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "Not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/bookings/reference/{reference} [get]
func (h *AppBookingHandler) GetBookingByReference(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	reference := c.Param("reference")
	booking, err := h.bookingRepo.GetBookingByReference(reference)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get booking"})
		return
	}

	// Check ownership
	if booking.UserID != userCtx.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to view this booking"})
		return
	}

	c.JSON(http.StatusOK, booking)
}

// ConfirmPayment confirms payment for a booking
// @Summary Confirm payment
// @Description Confirm payment for a booking
// @Tags App Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Param request body models.ConfirmAppPaymentRequest true "Payment confirmation"
// @Success 200 {object} map[string]interface{} "Payment confirmed"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "Not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/bookings/{id}/confirm-payment [post]
func (h *AppBookingHandler) ConfirmPayment(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	bookingID := c.Param("id")

	var req models.ConfirmAppPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get booking and verify ownership
	booking, err := h.bookingRepo.GetBookingByID(bookingID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get booking"})
		return
	}

	if booking.UserID != userCtx.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if booking.PaymentStatus == models.MasterPaymentPaid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment already confirmed"})
		return
	}

	// Update payment status
	gateway := req.PaymentGateway
	err = h.bookingRepo.UpdatePaymentStatus(
		bookingID,
		models.MasterPaymentPaid,
		&req.PaymentMethod,
		&req.PaymentReference,
		&gateway,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm payment"})
		return
	}

	// Also update bus booking and seat statuses to confirmed/booked
	// (This would be done by the repository in a proper implementation)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Payment confirmed successfully",
		"booking_id": bookingID,
		"status":     "confirmed",
	})
}

// CancelBooking cancels a booking
// @Summary Cancel booking
// @Description Cancel a booking and release seats
// @Tags App Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Param request body models.CancelAppBookingRequest true "Cancellation reason"
// @Success 200 {object} map[string]interface{} "Booking cancelled"
// @Failure 400 {object} map[string]interface{} "Cannot cancel"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "Not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/bookings/{id}/cancel [post]
func (h *AppBookingHandler) CancelBooking(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	bookingID := c.Param("id")

	var req models.CancelAppBookingRequest
	c.ShouldBindJSON(&req) // Reason is optional

	// Get booking and verify ownership
	booking, err := h.bookingRepo.GetBookingByID(bookingID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get booking"})
		return
	}

	if booking.UserID != userCtx.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if !booking.CanBeCancelled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Booking cannot be cancelled"})
		return
	}

	// Cancel booking
	reason := &req.Reason
	if req.Reason == "" {
		reason = nil
	}
	err = h.bookingRepo.CancelBooking(bookingID, userCtx.UserID.String(), reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel booking", "details": err.Error()})
		return
	}

	// Check if refund is needed
	refundNeeded := booking.IsPaid()

	c.JSON(http.StatusOK, gin.H{
		"message":       "Booking cancelled successfully",
		"booking_id":    bookingID,
		"refund_needed": refundNeeded,
		"refund_amount": booking.TotalAmount,
	})
}

// GetBookingQR retrieves QR code for a booking
// @Summary Get booking QR code
// @Description Get QR code data for boarding
// @Tags App Bookings
// @Produce json
// @Param id path string true "Booking ID"
// @Success 200 {object} map[string]interface{} "QR code data"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "Not found"
// @Security BearerAuth
// @Router /api/v1/bookings/{id}/qr [get]
func (h *AppBookingHandler) GetBookingQR(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	bookingID := c.Param("id")
	booking, err := h.bookingRepo.GetBookingByID(bookingID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get booking"})
		return
	}

	if booking.UserID != userCtx.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if booking.BusBooking == nil || booking.BusBooking.QRCodeData == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "QR code not available"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"qr_code":            *booking.BusBooking.QRCodeData,
		"booking_reference":  booking.BookingReference,
		"passenger_name":     booking.PassengerName,
		"route_name":         booking.BusBooking.RouteName,
		"departure_datetime": booking.BusBooking.DepartureDatetime,
		"seats":              len(booking.BusBooking.Seats),
	})
}
