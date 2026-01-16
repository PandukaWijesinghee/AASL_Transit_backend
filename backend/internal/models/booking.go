package models

import (
	"errors"
	"time"
)

// PaymentStatus represents the payment status of a booking
type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusPaid     PaymentStatus = "paid"
	PaymentStatusFailed   PaymentStatus = "failed"
	PaymentStatusRefunded PaymentStatus = "refunded"
)

// BookingStatus represents the status of a booking
type BookingStatus string

const (
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusCancelled BookingStatus = "cancelled"
	BookingStatusCompleted BookingStatus = "completed"
	BookingStatusNoShow    BookingStatus = "no_show"
)

// Booking represents a passenger trip reservation
type Booking struct {
	ID                 string        `json:"id" db:"id"`
	ScheduledTripID    string        `json:"scheduled_trip_id" db:"scheduled_trip_id"`
	UserID             string        `json:"user_id" db:"user_id"`
	BookingReference   string        `json:"booking_reference" db:"booking_reference"`
	NumberOfSeats      int           `json:"number_of_seats" db:"number_of_seats"`
	BoardingStopID     *string       `json:"boarding_stop_id,omitempty" db:"boarding_stop_id"`
	AlightingStopID    *string       `json:"alighting_stop_id,omitempty" db:"alighting_stop_id"`
	TotalFare          float64       `json:"total_fare" db:"total_fare"`
	PaymentStatus      PaymentStatus `json:"payment_status" db:"payment_status"`
	PaymentMethod      *string       `json:"payment_method,omitempty" db:"payment_method"`
	PaymentReference   *string       `json:"payment_reference,omitempty" db:"payment_reference"`
	PaidAt             *time.Time    `json:"paid_at,omitempty" db:"paid_at"`
	BookingStatus      BookingStatus `json:"booking_status" db:"booking_status"`
	CancelledAt        *time.Time    `json:"cancelled_at,omitempty" db:"cancelled_at"`
	CancellationReason *string       `json:"cancellation_reason,omitempty" db:"cancellation_reason"`
	PassengerName      *string       `json:"passenger_name,omitempty" db:"passenger_name"`
	PassengerPhone     *string       `json:"passenger_phone,omitempty" db:"passenger_phone"`
	PassengerEmail     *string       `json:"passenger_email,omitempty" db:"passenger_email"`
	SpecialRequests    *string       `json:"special_requests,omitempty" db:"special_requests"`
	CreatedAt          time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at" db:"updated_at"`
}

// CreateBookingRequest represents the request to create a booking
type CreateBookingRequest struct {
	ScheduledTripID  string  `json:"scheduled_trip_id" binding:"required"`
	NumberOfSeats    int     `json:"number_of_seats" binding:"required,min=1"`
	BoardingStopID   *string `json:"boarding_stop_id,omitempty"`
	AlightingStopID  *string `json:"alighting_stop_id,omitempty"`
	PassengerName    *string `json:"passenger_name,omitempty"`
	PassengerPhone   *string `json:"passenger_phone,omitempty"`
	PassengerEmail   *string `json:"passenger_email,omitempty"`
	SpecialRequests  *string `json:"special_requests,omitempty"`
	PaymentMethod    *string `json:"payment_method,omitempty"`
}

// CancelBookingRequest represents the request to cancel a booking
type CancelBookingRequest struct {
	CancellationReason *string `json:"cancellation_reason,omitempty"`
}

// ConfirmPaymentRequest represents the request to confirm payment
type ConfirmPaymentRequest struct {
	PaymentMethod    string `json:"payment_method" binding:"required"`
	PaymentReference string `json:"payment_reference" binding:"required"`
}

// Validate validates the create booking request
func (r *CreateBookingRequest) Validate() error {
	if r.NumberOfSeats <= 0 {
		return errors.New("number_of_seats must be at least 1")
	}

	if r.NumberOfSeats > 10 {
		return errors.New("maximum 10 seats can be booked at once")
	}

	return nil
}

// CanBeCancelled checks if the booking can be cancelled
func (b *Booking) CanBeCancelled() bool {
	return b.BookingStatus == BookingStatusConfirmed
}

// Cancel cancels the booking
func (b *Booking) Cancel(reason *string) error {
	if !b.CanBeCancelled() {
		return errors.New("booking cannot be cancelled")
	}

	now := time.Now()
	b.BookingStatus = BookingStatusCancelled
	b.CancelledAt = &now
	b.CancellationReason = reason
	b.UpdatedAt = now

	return nil
}

// ConfirmPayment confirms payment for the booking
func (b *Booking) ConfirmPayment(method, reference string) error {
	if b.PaymentStatus == PaymentStatusPaid {
		return errors.New("payment already confirmed")
	}

	now := time.Now()
	b.PaymentStatus = PaymentStatusPaid
	b.PaymentMethod = &method
	b.PaymentReference = &reference
	b.PaidAt = &now
	b.UpdatedAt = now

	return nil
}

// MarkAsCompleted marks the booking as completed
func (b *Booking) MarkAsCompleted() {
	b.BookingStatus = BookingStatusCompleted
	b.UpdatedAt = time.Now()
}

// MarkAsNoShow marks the booking as no-show
func (b *Booking) MarkAsNoShow() {
	b.BookingStatus = BookingStatusNoShow
	b.UpdatedAt = time.Now()
}

// IsActive checks if the booking is active (confirmed and paid)
func (b *Booking) IsActive() bool {
	return b.BookingStatus == BookingStatusConfirmed && b.PaymentStatus == PaymentStatusPaid
}

// IsPaid checks if the booking is paid
func (b *Booking) IsPaid() bool {
	return b.PaymentStatus == PaymentStatusPaid
}

// NeedsRefund checks if the booking needs a refund
func (b *Booking) NeedsRefund() bool {
	return b.BookingStatus == BookingStatusCancelled && b.PaymentStatus == PaymentStatusPaid
}

// CalculateRefundAmount calculates the refund amount based on cancellation time
func (b *Booking) CalculateRefundAmount(tripDateTime time.Time) float64 {
	if !b.NeedsRefund() {
		return 0
	}

	if b.CancelledAt == nil {
		return 0
	}

	hoursBeforeTrip := tripDateTime.Sub(*b.CancelledAt).Hours()

	// Refund policy:
	// - More than 24 hours before: 100% refund
	// - 12-24 hours before: 75% refund
	// - 6-12 hours before: 50% refund
	// - Less than 6 hours: 25% refund

	if hoursBeforeTrip >= 24 {
		return b.TotalFare
	} else if hoursBeforeTrip >= 12 {
		return b.TotalFare * 0.75
	} else if hoursBeforeTrip >= 6 {
		return b.TotalFare * 0.50
	} else {
		return b.TotalFare * 0.25
	}
}
