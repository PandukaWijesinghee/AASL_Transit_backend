package models

import (
	"time"
)

// TripSeatStatus represents the status of a trip seat
type TripSeatStatus string

const (
	TripSeatStatusAvailable TripSeatStatus = "available"
	TripSeatStatusReserved  TripSeatStatus = "reserved"
	TripSeatStatusBooked    TripSeatStatus = "booked"
	TripSeatStatusBlocked   TripSeatStatus = "blocked"
)

// TripSeatBookingType represents how a seat was booked
type TripSeatBookingType string

const (
	TripSeatBookingTypeApp     TripSeatBookingType = "app"
	TripSeatBookingTypePhone   TripSeatBookingType = "phone"
	TripSeatBookingTypeAgent   TripSeatBookingType = "agent"
	TripSeatBookingTypeWalkIn  TripSeatBookingType = "walk_in"
	TripSeatBookingTypeBlocked TripSeatBookingType = "blocked"
)

// TripSeat represents a seat for a specific scheduled trip
type TripSeat struct {
	ID               string              `json:"id" db:"id"`
	ScheduledTripID  string              `json:"scheduled_trip_id" db:"scheduled_trip_id"`
	SeatNumber       string              `json:"seat_number" db:"seat_number"`
	SeatType         string              `json:"seat_type" db:"seat_type"` // standard, window, aisle, premium, accessible
	RowNumber        int                 `json:"row_number" db:"row_number"`
	Position         int                 `json:"position" db:"position"`
	SeatPrice        float64             `json:"seat_price" db:"seat_price"`
	Status           TripSeatStatus      `json:"status" db:"status"`
	BookingType      *TripSeatBookingType `json:"booking_type,omitempty" db:"booking_type"`
	BusBookingSeatID *string             `json:"bus_booking_seat_id,omitempty" db:"bus_booking_seat_id"`
	ManualBookingID  *string             `json:"manual_booking_id,omitempty" db:"manual_booking_id"`
	BlockReason      *string             `json:"block_reason,omitempty" db:"block_reason"`
	BlockedByUserID  *string             `json:"blocked_by_user_id,omitempty" db:"blocked_by_user_id"`
	BlockedAt        *time.Time          `json:"blocked_at,omitempty" db:"blocked_at"`
	CreatedAt        time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at" db:"updated_at"`
}

// TripSeatSummary provides a quick overview of seat availability for a trip
type TripSeatSummary struct {
	ScheduledTripID   string `json:"scheduled_trip_id" db:"scheduled_trip_id"`
	TotalSeats        int    `json:"total_seats" db:"total_seats"`
	AvailableSeats    int    `json:"available_seats" db:"available_seats"`
	BookedSeats       int    `json:"booked_seats" db:"booked_seats"`
	BlockedSeats      int    `json:"blocked_seats" db:"blocked_seats"`
	ReservedSeats     int    `json:"reserved_seats" db:"reserved_seats"`
	AppBookings       int    `json:"app_bookings" db:"app_bookings"`
	PhoneBookings     int    `json:"phone_bookings" db:"phone_bookings"`
	AgentBookings     int    `json:"agent_bookings" db:"agent_bookings"`
	WalkInBookings    int    `json:"walk_in_bookings" db:"walk_in_bookings"`
}

// TripSeatWithBookingInfo includes booking details for display
type TripSeatWithBookingInfo struct {
	TripSeat
	// From manual_seat_bookings (if manual booking)
	PassengerName  *string `json:"passenger_name,omitempty" db:"passenger_name"`
	PassengerPhone *string `json:"passenger_phone,omitempty" db:"passenger_phone"`
	BookingRef     *string `json:"booking_reference,omitempty" db:"booking_reference"`
	PaymentStatus  *string `json:"payment_status,omitempty" db:"payment_status"`
}

// CreateTripSeatsRequest is used when assigning a seat layout to a trip
type CreateTripSeatsRequest struct {
	ScheduledTripID string  `json:"scheduled_trip_id" binding:"required"`
	SeatLayoutID    string  `json:"seat_layout_id" binding:"required"`
	BaseFare        float64 `json:"base_fare" binding:"required,gte=0"`
}

// BlockSeatsRequest is used to block one or more seats
type BlockSeatsRequest struct {
	SeatIDs []string `json:"seat_ids" binding:"required,min=1"`
	Reason  string   `json:"reason"`
}

// UnblockSeatsRequest is used to unblock one or more seats
type UnblockSeatsRequest struct {
	SeatIDs []string `json:"seat_ids" binding:"required,min=1"`
}

// UpdateSeatPriceRequest is used to update price for specific seats
type UpdateSeatPriceRequest struct {
	SeatIDs  []string `json:"seat_ids" binding:"required,min=1"`
	NewPrice float64  `json:"new_price" binding:"required,gte=0"`
}
