package models

import (
	"fmt"
	"time"
)

// ManualBookingType represents the type of manual booking
type ManualBookingType string

const (
	ManualBookingTypePhone  ManualBookingType = "phone"
	ManualBookingTypeAgent  ManualBookingType = "agent"
	ManualBookingTypeWalkIn ManualBookingType = "walk_in"
)

// ManualBookingPaymentStatus represents the payment status
type ManualBookingPaymentStatus string

const (
	ManualBookingPaymentPending      ManualBookingPaymentStatus = "pending"
	ManualBookingPaymentPartial      ManualBookingPaymentStatus = "partial"
	ManualBookingPaymentPaid         ManualBookingPaymentStatus = "paid"
	ManualBookingPaymentCollectOnBus ManualBookingPaymentStatus = "collect_on_bus"
	ManualBookingPaymentFree         ManualBookingPaymentStatus = "free"
)

// ManualBookingStatus represents the booking status
type ManualBookingStatus string

const (
	ManualBookingStatusConfirmed ManualBookingStatus = "confirmed"
	ManualBookingStatusCheckedIn ManualBookingStatus = "checked_in"
	ManualBookingStatusBoarded   ManualBookingStatus = "boarded"
	ManualBookingStatusCompleted ManualBookingStatus = "completed"
	ManualBookingStatusCancelled ManualBookingStatus = "cancelled"
	ManualBookingStatusNoShow    ManualBookingStatus = "no_show"
)

// ManualSeatBooking represents a phone/agent/walk-in booking
type ManualSeatBooking struct {
	ID                 string                     `json:"id" db:"id"`
	BookingReference   string                     `json:"booking_reference" db:"booking_reference"`
	ScheduledTripID    string                     `json:"scheduled_trip_id" db:"scheduled_trip_id"`
	CreatedByUserID    string                     `json:"created_by_user_id" db:"created_by_user_id"`
	BookingType        ManualBookingType          `json:"booking_type" db:"booking_type"`
	PassengerName      string                     `json:"passenger_name" db:"passenger_name"`
	PassengerPhone     *string                    `json:"passenger_phone,omitempty" db:"passenger_phone"`
	PassengerNIC       *string                    `json:"passenger_nic,omitempty" db:"passenger_nic"`
	PassengerNotes     *string                    `json:"passenger_notes,omitempty" db:"passenger_notes"`
	BoardingStopID     *string                    `json:"boarding_stop_id,omitempty" db:"boarding_stop_id"`
	AlightingStopID    *string                    `json:"alighting_stop_id,omitempty" db:"alighting_stop_id"`
	DepartureDatetime  time.Time                  `json:"departure_datetime" db:"departure_datetime"`
	NumberOfSeats      int                        `json:"number_of_seats" db:"number_of_seats"`
	TotalFare          float64                    `json:"total_fare" db:"total_fare"`
	PaymentStatus      ManualBookingPaymentStatus `json:"payment_status" db:"payment_status"`
	AmountPaid         float64                    `json:"amount_paid" db:"amount_paid"`
	PaymentMethod      *string                    `json:"payment_method,omitempty" db:"payment_method"`
	PaymentNotes       *string                    `json:"payment_notes,omitempty" db:"payment_notes"`
	Status             ManualBookingStatus        `json:"status" db:"status"`
	ConfirmedAt        *time.Time                 `json:"confirmed_at,omitempty" db:"confirmed_at"`
	CheckedInAt        *time.Time                 `json:"checked_in_at,omitempty" db:"checked_in_at"`
	BoardedAt          *time.Time                 `json:"boarded_at,omitempty" db:"boarded_at"`
	CompletedAt        *time.Time                 `json:"completed_at,omitempty" db:"completed_at"`
	CancelledAt        *time.Time                 `json:"cancelled_at,omitempty" db:"cancelled_at"`
	CancellationReason *string                    `json:"cancellation_reason,omitempty" db:"cancellation_reason"`
	CreatedAt          time.Time                  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at" db:"updated_at"`
	// Populated from joins (not stored in DB, but scanned from query results)
	RouteName         string `json:"route_name,omitempty" db:"route_name"`
	BoardingStopName  string `json:"boarding_stop_name,omitempty" db:"boarding_stop_name"`
	AlightingStopName string `json:"alighting_stop_name,omitempty" db:"alighting_stop_name"`
}

// ManualBookingSeat represents a seat in a manual booking
type ManualBookingSeat struct {
	ID              string    `json:"id" db:"id"`
	ManualBookingID string    `json:"manual_booking_id" db:"manual_booking_id"`
	TripSeatID      string    `json:"trip_seat_id" db:"trip_seat_id"`
	SeatNumber      string    `json:"seat_number" db:"seat_number"`
	SeatPrice       float64   `json:"seat_price" db:"seat_price"`
	PassengerName   *string   `json:"passenger_name,omitempty" db:"passenger_name"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// ManualBookingWithSeats includes the booking and its seats
type ManualBookingWithSeats struct {
	ManualSeatBooking
	Seats []ManualBookingSeat `json:"seats"`
}

// CreateManualBookingRequest is the request to create a phone/agent booking
type CreateManualBookingRequest struct {
	ScheduledTripID string   `json:"scheduled_trip_id"` // Set from URL path, not required in body
	BookingType     string   `json:"booking_type" binding:"required,oneof=phone agent walk_in"`
	PassengerName   string   `json:"passenger_name" binding:"required"`
	PassengerPhone  *string  `json:"passenger_phone,omitempty"`
	PassengerNIC    *string  `json:"passenger_nic,omitempty"`
	PassengerNotes  *string  `json:"passenger_notes,omitempty"`
	BoardingStopID  string   `json:"boarding_stop_id" binding:"required,uuid"`  // Required - master_route_stops ID
	AlightingStopID string   `json:"alighting_stop_id" binding:"required,uuid"` // Required - master_route_stops ID
	SeatIDs         []string `json:"seat_ids" binding:"required,min=1"`         // trip_seat IDs
	PaymentStatus   string   `json:"payment_status" binding:"required,oneof=pending partial paid collect_on_bus free"`
	AmountPaid      float64  `json:"amount_paid"`
	PaymentMethod   *string  `json:"payment_method,omitempty"`
	PaymentNotes    *string  `json:"payment_notes,omitempty"`
}

// UpdateManualBookingPaymentRequest updates payment info
type UpdateManualBookingPaymentRequest struct {
	PaymentStatus string  `json:"payment_status" binding:"required,oneof=pending partial paid collect_on_bus free"`
	AmountPaid    float64 `json:"amount_paid"`
	PaymentMethod *string `json:"payment_method,omitempty"`
	PaymentNotes  *string `json:"payment_notes,omitempty"`
}

// CancelManualBookingRequest cancels a manual booking
type CancelManualBookingRequest struct {
	Reason string `json:"reason"`
}

// GenerateBookingReference generates a unique booking reference
// Format: PH-20251206-001, AG-20251206-001, WI-20251206-001
func GenerateBookingReference(bookingType ManualBookingType, sequenceNum int) string {
	prefix := "MB"
	switch bookingType {
	case ManualBookingTypePhone:
		prefix = "PH"
	case ManualBookingTypeAgent:
		prefix = "AG"
	case ManualBookingTypeWalkIn:
		prefix = "WI"
	}

	datePart := time.Now().Format("20060102")
	return fmt.Sprintf("%s-%s-%03d", prefix, datePart, sequenceNum)
}
