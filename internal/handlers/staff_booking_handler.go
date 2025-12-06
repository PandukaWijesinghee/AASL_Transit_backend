package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
)

// StaffBookingHandler handles conductor/driver booking operations
type StaffBookingHandler struct {
	bookingRepo *database.AppBookingRepository
}

// NewStaffBookingHandler creates a new StaffBookingHandler
func NewStaffBookingHandler(bookingRepo *database.AppBookingRepository) *StaffBookingHandler {
	return &StaffBookingHandler{bookingRepo: bookingRepo}
}

// VerifyBookingRequest represents a request to verify a booking by QR
type VerifyBookingRequest struct {
	QRCode string `json:"qr_code" binding:"required"`
}

// VerifyBookingByQR verifies a booking by scanning QR code
// @Summary Verify booking by QR
// @Description Conductor/Driver scans QR to verify booking
// @Tags Staff Bookings
// @Accept json
// @Produce json
// @Param request body VerifyBookingRequest true "QR code data"
// @Success 200 {object} map[string]interface{} "Booking details"
// @Failure 400 {object} map[string]interface{} "Invalid QR"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Booking not found"
// @Security BearerAuth
// @Router /api/v1/staff/bookings/verify [post]
func (h *StaffBookingHandler) VerifyBookingByQR(c *gin.Context) {
	_, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// TODO: Add role check for conductor/driver

	var req VerifyBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	busBooking, err := h.bookingRepo.GetBusBookingByQRCode(req.QRCode)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify booking"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":              true,
		"bus_booking_id":     busBooking.ID,
		"route_name":         busBooking.RouteName,
		"boarding_stop":      busBooking.BoardingStopName,
		"alighting_stop":     busBooking.AlightingStopName,
		"departure_datetime": busBooking.DepartureDatetime,
		"number_of_seats":    busBooking.NumberOfSeats,
		"status":             busBooking.Status,
		"is_checked_in":      busBooking.CheckedInAt != nil,
		"check_in_time":      busBooking.CheckedInAt,
		"seats":              busBooking.Seats,
	})
}

// CheckInRequest represents a check-in request
type CheckInRequest struct {
	BusBookingID string `json:"bus_booking_id" binding:"required"`
	// Optional: specific seat to check in
	SeatID string `json:"seat_id,omitempty"`
}

// CheckInPassenger marks passenger as checked-in
// @Summary Check in passenger
// @Description Conductor marks passenger as checked-in (verified ticket)
// @Tags Staff Bookings
// @Accept json
// @Produce json
// @Param request body CheckInRequest true "Check-in details"
// @Success 200 {object} map[string]interface{} "Checked in successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/staff/bookings/check-in [post]
func (h *StaffBookingHandler) CheckInPassenger(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// If specific seat, check in that seat
	if req.SeatID != "" {
		err := h.bookingRepo.CheckInPassenger(req.SeatID, userCtx.UserID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check in", "details": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Seat checked in successfully",
			"seat_id": req.SeatID,
		})
		return
	}

	// Otherwise check in the whole bus booking
	err := h.bookingRepo.CheckInBusBooking(req.BusBookingID, userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check in", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Booking checked in successfully",
		"bus_booking_id": req.BusBookingID,
	})
}

// BoardRequest represents a boarding request
type BoardRequest struct {
	SeatID string `json:"seat_id" binding:"required"`
}

// BoardPassenger marks passenger as boarded
// @Summary Board passenger
// @Description Conductor marks passenger as boarded (on the bus)
// @Tags Staff Bookings
// @Accept json
// @Produce json
// @Param request body BoardRequest true "Boarding details"
// @Success 200 {object} map[string]interface{} "Boarded successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/staff/bookings/board [post]
func (h *StaffBookingHandler) BoardPassenger(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req BoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	err := h.bookingRepo.BoardPassenger(req.SeatID, userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to board passenger", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Passenger boarded successfully",
		"seat_id": req.SeatID,
	})
}

// NoShowRequest represents a no-show request
type NoShowRequest struct {
	SeatID string `json:"seat_id" binding:"required"`
}

// MarkNoShow marks passenger as no-show
// @Summary Mark no-show
// @Description Conductor marks passenger as no-show
// @Tags Staff Bookings
// @Accept json
// @Produce json
// @Param request body NoShowRequest true "No-show details"
// @Success 200 {object} map[string]interface{} "Marked as no-show"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/staff/bookings/no-show [post]
func (h *StaffBookingHandler) MarkNoShow(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req NoShowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	err := h.bookingRepo.MarkNoShow(req.SeatID, userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark no-show", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Passenger marked as no-show",
		"seat_id": req.SeatID,
	})
}

// GetTripBookings gets all bookings for a trip
// @Summary Get trip bookings
// @Description Get all bookings for a scheduled trip (for staff)
// @Tags Staff Bookings
// @Produce json
// @Param trip_id path string true "Scheduled Trip ID"
// @Success 200 {array} models.BusBooking "List of bookings"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /api/v1/staff/trips/{trip_id}/bookings [get]
func (h *StaffBookingHandler) GetTripBookings(c *gin.Context) {
	_, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tripID := c.Param("trip_id")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	bookings, err := h.bookingRepo.GetBusBookingsByTripID(tripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bookings"})
		return
	}

	// Calculate stats
	var totalBooked, checkedIn, boarded, noShow int
	for _, b := range bookings {
		totalBooked += b.NumberOfSeats
		if b.CheckedInAt != nil {
			checkedIn += b.NumberOfSeats
		}
		for _, seat := range b.Seats {
			if seat.BoardedAt != nil {
				boarded++
			}
			if seat.Status == "no_show" {
				noShow++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"bookings":      bookings,
		"total_booked":  totalBooked,
		"checked_in":    checkedIn,
		"boarded":       boarded,
		"no_show":       noShow,
		"booking_count": len(bookings),
	})
}
