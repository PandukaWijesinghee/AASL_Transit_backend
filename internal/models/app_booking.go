package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ============================================================================
// BOOKING TYPES & STATUSES
// ============================================================================

// BookingType represents the type of booking
type BookingType string

const (
	BookingTypeBusOnly       BookingType = "bus_only"
	BookingTypeLoungeOnly    BookingType = "lounge_only"
	BookingTypeBusWithLounge BookingType = "bus_with_lounge"
)

// MasterPaymentStatus represents payment status for the master booking
type MasterPaymentStatus string

const (
	MasterPaymentPending       MasterPaymentStatus = "pending"
	MasterPaymentPartial       MasterPaymentStatus = "partial"
	MasterPaymentPaid          MasterPaymentStatus = "paid"
	MasterPaymentFailed        MasterPaymentStatus = "failed"
	MasterPaymentRefunded      MasterPaymentStatus = "refunded"
	MasterPaymentPartialRefund MasterPaymentStatus = "partial_refund"
)

// MasterBookingStatus represents overall booking status
type MasterBookingStatus string

const (
	MasterBookingPending       MasterBookingStatus = "pending"
	MasterBookingConfirmed     MasterBookingStatus = "confirmed"
	MasterBookingInProgress    MasterBookingStatus = "in_progress"
	MasterBookingCompleted     MasterBookingStatus = "completed"
	MasterBookingCancelled     MasterBookingStatus = "cancelled"
	MasterBookingPartialCancel MasterBookingStatus = "partial_cancel"
)

// BookingSource represents where the booking originated
type BookingSource string

const (
	BookingSourceApp   BookingSource = "app"
	BookingSourceWeb   BookingSource = "web"
	BookingSourceAgent BookingSource = "agent"
	BookingSourceKiosk BookingSource = "kiosk"
)

// BusBookingStatus represents bus booking status
type BusBookingStatus string

const (
	BusBookingPending   BusBookingStatus = "pending"
	BusBookingConfirmed BusBookingStatus = "confirmed"
	BusBookingCheckedIn BusBookingStatus = "checked_in"
	BusBookingBoarded   BusBookingStatus = "boarded"
	BusBookingInTransit BusBookingStatus = "in_transit"
	BusBookingCompleted BusBookingStatus = "completed"
	BusBookingCancelled BusBookingStatus = "cancelled"
	BusBookingNoShow    BusBookingStatus = "no_show"
)

// SeatBookingStatus represents individual seat status
type SeatBookingStatus string

const (
	SeatBookingPending   SeatBookingStatus = "pending"
	SeatBookingBooked    SeatBookingStatus = "booked"
	SeatBookingCheckedIn SeatBookingStatus = "checked_in"
	SeatBookingBoarded   SeatBookingStatus = "boarded"
	SeatBookingCompleted SeatBookingStatus = "completed"
	SeatBookingCancelled SeatBookingStatus = "cancelled"
	SeatBookingNoShow    SeatBookingStatus = "no_show"
)

// ============================================================================
// MASTER BOOKING (bookings table)
// ============================================================================

// DeviceInfo stores device metadata
type DeviceInfo map[string]interface{}

func (d DeviceInfo) Value() (driver.Value, error) {
	return json.Marshal(d)
}

func (d *DeviceInfo) Scan(value interface{}) error {
	if value == nil {
		*d = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, d)
}

// MasterBooking represents the main booking record (bookings table)
type MasterBooking struct {
	ID               string      `json:"id" db:"id"`
	BookingReference string      `json:"booking_reference" db:"booking_reference"`
	UserID           string      `json:"user_id" db:"user_id"`
	BookingType      BookingType `json:"booking_type" db:"booking_type"`

	// Totals
	BusTotal       float64 `json:"bus_total" db:"bus_total"`
	LoungeTotal    float64 `json:"lounge_total" db:"lounge_total"`
	PreOrderTotal  float64 `json:"pre_order_total" db:"pre_order_total"`
	Subtotal       float64 `json:"subtotal" db:"subtotal"`
	DiscountAmount float64 `json:"discount_amount" db:"discount_amount"`
	TaxAmount      float64 `json:"tax_amount" db:"tax_amount"`
	TotalAmount    float64 `json:"total_amount" db:"total_amount"`

	// Promo
	PromoCode          *string `json:"promo_code,omitempty" db:"promo_code"`
	PromoDiscountType  *string `json:"promo_discount_type,omitempty" db:"promo_discount_type"`
	PromoDiscountValue float64 `json:"promo_discount_value" db:"promo_discount_value"`

	// Payment
	PaymentStatus    MasterPaymentStatus `json:"payment_status" db:"payment_status"`
	PaymentMethod    *string             `json:"payment_method,omitempty" db:"payment_method"`
	PaymentReference *string             `json:"payment_reference,omitempty" db:"payment_reference"`
	PaymentGateway   *string             `json:"payment_gateway,omitempty" db:"payment_gateway"`
	PaidAt           *time.Time          `json:"paid_at,omitempty" db:"paid_at"`

	// Status
	BookingStatus MasterBookingStatus `json:"booking_status" db:"booking_status"`

	// Contact
	PassengerName  string  `json:"passenger_name" db:"passenger_name"`
	PassengerPhone string  `json:"passenger_phone" db:"passenger_phone"`
	PassengerEmail *string `json:"passenger_email,omitempty" db:"passenger_email"`

	// Timestamps
	ConfirmedAt        *time.Time `json:"confirmed_at,omitempty" db:"confirmed_at"`
	CancelledAt        *time.Time `json:"cancelled_at,omitempty" db:"cancelled_at"`
	CancellationReason *string    `json:"cancellation_reason,omitempty" db:"cancellation_reason"`
	CancelledByUserID  *string    `json:"cancelled_by_user_id,omitempty" db:"cancelled_by_user_id"`
	CompletedAt        *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// Refund
	RefundAmount    float64    `json:"refund_amount" db:"refund_amount"`
	RefundReference *string    `json:"refund_reference,omitempty" db:"refund_reference"`
	RefundedAt      *time.Time `json:"refunded_at,omitempty" db:"refunded_at"`

	// Metadata
	BookingSource BookingSource `json:"booking_source" db:"booking_source"`
	DeviceInfo    DeviceInfo    `json:"device_info,omitempty" db:"device_info"`
	Notes         *string       `json:"notes,omitempty" db:"notes"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Related data (not in DB, populated by queries)
	BusBooking *BusBooking `json:"bus_booking,omitempty" db:"-"`
	// LoungeBookings   []LoungeBooking     `json:"lounge_bookings,omitempty" db:"-"` // Future
}

// ============================================================================
// BUS BOOKING (bus_bookings table)
// ============================================================================

// BusBooking represents bus trip details within a booking
type BusBooking struct {
	ID              string `json:"id" db:"id"`
	BookingID       string `json:"booking_id" db:"booking_id"`
	ScheduledTripID string `json:"scheduled_trip_id" db:"scheduled_trip_id"`

	// Trip Snapshot
	RouteName  string  `json:"route_name" db:"route_name"`
	BusNumber  *string `json:"bus_number,omitempty" db:"bus_number"`
	BusType    *string `json:"bus_type,omitempty" db:"bus_type"`
	BusOwnerID *string `json:"bus_owner_id,omitempty" db:"bus_owner_id"`

	// Stops
	BoardingStopID     *string `json:"boarding_stop_id,omitempty" db:"boarding_stop_id"`
	BoardingStopName   string  `json:"boarding_stop_name" db:"boarding_stop_name"`
	BoardingStopOrder  *int    `json:"boarding_stop_order,omitempty" db:"boarding_stop_order"`
	AlightingStopID    *string `json:"alighting_stop_id,omitempty" db:"alighting_stop_id"`
	AlightingStopName  string  `json:"alighting_stop_name" db:"alighting_stop_name"`
	AlightingStopOrder *int    `json:"alighting_stop_order,omitempty" db:"alighting_stop_order"`

	// Timing
	DepartureDatetime        time.Time  `json:"departure_datetime" db:"departure_datetime"`
	EstimatedArrivalDatetime *time.Time `json:"estimated_arrival_datetime,omitempty" db:"estimated_arrival_datetime"`
	ActualDepartureDatetime  *time.Time `json:"actual_departure_datetime,omitempty" db:"actual_departure_datetime"`
	ActualArrivalDatetime    *time.Time `json:"actual_arrival_datetime,omitempty" db:"actual_arrival_datetime"`

	// Seats & Fare
	NumberOfSeats int     `json:"number_of_seats" db:"number_of_seats"`
	FarePerSeat   float64 `json:"fare_per_seat" db:"fare_per_seat"`
	TotalFare     float64 `json:"total_fare" db:"total_fare"`

	// Status
	Status BusBookingStatus `json:"status" db:"status"`

	// Check-in/Boarding
	CheckedInAt       *time.Time `json:"checked_in_at,omitempty" db:"checked_in_at"`
	CheckedInByUserID *string    `json:"checked_in_by_user_id,omitempty" db:"checked_in_by_user_id"`
	BoardedAt         *time.Time `json:"boarded_at,omitempty" db:"boarded_at"`
	BoardedByUserID   *string    `json:"boarded_by_user_id,omitempty" db:"boarded_by_user_id"`
	CompletedAt       *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// Cancellation
	CancelledAt        *time.Time `json:"cancelled_at,omitempty" db:"cancelled_at"`
	CancellationReason *string    `json:"cancellation_reason,omitempty" db:"cancellation_reason"`

	// QR Code
	QRCodeData    *string    `json:"qr_code_data,omitempty" db:"qr_code_data"`
	QRGeneratedAt *time.Time `json:"qr_generated_at,omitempty" db:"qr_generated_at"`

	SpecialRequests *string `json:"special_requests,omitempty" db:"special_requests"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Related data
	Seats []BusBookingSeat `json:"seats,omitempty" db:"-"`
}

// ============================================================================
// BUS BOOKING SEAT (bus_booking_seats table)
// ============================================================================

// BusBookingSeat represents an individual seat in a bus booking
type BusBookingSeat struct {
	ID              string  `json:"id" db:"id"`
	BusBookingID    string  `json:"bus_booking_id" db:"bus_booking_id"`
	ScheduledTripID string  `json:"scheduled_trip_id" db:"scheduled_trip_id"`
	TripSeatID      *string `json:"trip_seat_id,omitempty" db:"trip_seat_id"`

	// Seat Info
	SeatNumber string  `json:"seat_number" db:"seat_number"`
	SeatType   string  `json:"seat_type" db:"seat_type"`
	SeatPrice  float64 `json:"seat_price" db:"seat_price"`

	// Passenger
	PassengerName      string  `json:"passenger_name" db:"passenger_name"`
	PassengerPhone     *string `json:"passenger_phone,omitempty" db:"passenger_phone"`
	PassengerEmail     *string `json:"passenger_email,omitempty" db:"passenger_email"`
	PassengerAge       *int    `json:"passenger_age,omitempty" db:"passenger_age"`
	PassengerGender    *string `json:"passenger_gender,omitempty" db:"passenger_gender"`
	PassengerNIC       *string `json:"passenger_nic,omitempty" db:"passenger_nic"`
	IsPrimaryPassenger bool    `json:"is_primary_passenger" db:"is_primary_passenger"`

	// Status
	Status SeatBookingStatus `json:"status" db:"status"`

	// Timestamps
	CheckedInAt *time.Time `json:"checked_in_at,omitempty" db:"checked_in_at"`
	BoardedAt   *time.Time `json:"boarded_at,omitempty" db:"boarded_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty" db:"cancelled_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ============================================================================
// REQUEST/RESPONSE STRUCTS
// ============================================================================

// SeatSelection represents a seat being booked with passenger details
type SeatSelection struct {
	TripSeatID      string  `json:"trip_seat_id" binding:"required"`
	SeatNumber      string  `json:"seat_number" binding:"required"`
	SeatType        string  `json:"seat_type"`
	SeatPrice       float64 `json:"seat_price"`
	PassengerName   string  `json:"passenger_name" binding:"required"`
	PassengerPhone  *string `json:"passenger_phone,omitempty"`
	PassengerEmail  *string `json:"passenger_email,omitempty"`
	PassengerAge    *int    `json:"passenger_age,omitempty"`
	PassengerGender *string `json:"passenger_gender,omitempty"`
	PassengerNIC    *string `json:"passenger_nic,omitempty"`
	IsPrimary       bool    `json:"is_primary"`
}

// CreateAppBookingRequest is the request to create a bus booking via app
type CreateAppBookingRequest struct {
	// Trip Info
	ScheduledTripID string `json:"scheduled_trip_id" binding:"required"`

	// Stops
	BoardingStopID    *string `json:"boarding_stop_id,omitempty"`
	BoardingStopName  string  `json:"boarding_stop_name" binding:"required"`
	AlightingStopID   *string `json:"alighting_stop_id,omitempty"`
	AlightingStopName string  `json:"alighting_stop_name" binding:"required"`

	// Seats with passenger details
	Seats []SeatSelection `json:"seats" binding:"required,min=1"`

	// Primary Contact (can be different from logged-in user)
	PassengerName  string  `json:"passenger_name" binding:"required"`
	PassengerPhone string  `json:"passenger_phone" binding:"required"`
	PassengerEmail *string `json:"passenger_email,omitempty"`

	// Payment
	PaymentMethod *string `json:"payment_method,omitempty"`

	// Promo
	PromoCode *string `json:"promo_code,omitempty"`

	// Special Requests
	SpecialRequests *string `json:"special_requests,omitempty"`

	// Device Info
	DeviceInfo DeviceInfo `json:"device_info,omitempty"`
}

// Validate validates the booking request
func (r *CreateAppBookingRequest) Validate() error {
	if len(r.Seats) == 0 {
		return errors.New("at least one seat must be selected")
	}
	if len(r.Seats) > 10 {
		return errors.New("maximum 10 seats can be booked at once")
	}

	// Ensure at least one primary passenger
	hasPrimary := false
	for _, seat := range r.Seats {
		if seat.IsPrimary {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary && len(r.Seats) > 0 {
		// Auto-set first seat as primary
		r.Seats[0].IsPrimary = true
	}

	return nil
}

// ConfirmAppPaymentRequest confirms payment for a booking
type ConfirmAppPaymentRequest struct {
	PaymentMethod    string `json:"payment_method" binding:"required"`
	PaymentReference string `json:"payment_reference" binding:"required"`
	PaymentGateway   string `json:"payment_gateway"`
}

// CancelAppBookingRequest cancels a booking
type CancelAppBookingRequest struct {
	Reason string `json:"reason"`
}

// BookingResponse is the response after creating a booking
type BookingResponse struct {
	Booking    *MasterBooking   `json:"booking"`
	BusBooking *BusBooking      `json:"bus_booking,omitempty"`
	Seats      []BusBookingSeat `json:"seats,omitempty"`
	QRCode     string           `json:"qr_code,omitempty"`
}

// BookingListItem is a summary for listing bookings
type BookingListItem struct {
	ID               string              `json:"id" db:"id"`
	BookingReference string              `json:"booking_reference" db:"booking_reference"`
	BookingType      BookingType         `json:"booking_type" db:"booking_type"`
	TotalAmount      float64             `json:"total_amount" db:"total_amount"`
	PaymentStatus    MasterPaymentStatus `json:"payment_status" db:"payment_status"`
	BookingStatus    MasterBookingStatus `json:"booking_status" db:"booking_status"`
	PassengerName    string              `json:"passenger_name" db:"passenger_name"`
	CreatedAt        time.Time           `json:"created_at" db:"created_at"`

	// Bus details (if applicable)
	RouteName         *string           `json:"route_name,omitempty" db:"route_name"`
	DepartureDatetime *time.Time        `json:"departure_datetime,omitempty" db:"departure_datetime"`
	NumberOfSeats     *int              `json:"number_of_seats,omitempty" db:"number_of_seats"`
	BusStatus         *BusBookingStatus `json:"bus_status,omitempty" db:"bus_status"`
	QRCodeData        *string           `json:"qr_code_data,omitempty" db:"qr_code_data"`
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// CanBeCancelled checks if booking can be cancelled
func (b *MasterBooking) CanBeCancelled() bool {
	return b.BookingStatus == MasterBookingPending ||
		b.BookingStatus == MasterBookingConfirmed
}

// IsPaid checks if booking is paid
func (b *MasterBooking) IsPaid() bool {
	return b.PaymentStatus == MasterPaymentPaid
}

// NeedsRefund checks if booking needs refund on cancellation
func (b *MasterBooking) NeedsRefund() bool {
	return b.BookingStatus == MasterBookingCancelled &&
		b.PaymentStatus == MasterPaymentPaid &&
		b.RefundAmount == 0
}

// CalculateTotals recalculates booking totals
func (b *MasterBooking) CalculateTotals() {
	b.Subtotal = b.BusTotal + b.LoungeTotal + b.PreOrderTotal
	b.TotalAmount = b.Subtotal - b.DiscountAmount + b.TaxAmount
}
