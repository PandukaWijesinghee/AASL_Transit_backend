package models

import (
	"errors"
	"time"
)

// ScheduledTripStatus represents the status of a scheduled trip
type ScheduledTripStatus string

const (
	ScheduledTripStatusScheduled  ScheduledTripStatus = "scheduled"
	ScheduledTripStatusConfirmed  ScheduledTripStatus = "confirmed"
	ScheduledTripStatusInProgress ScheduledTripStatus = "in_progress"
	ScheduledTripStatusCompleted  ScheduledTripStatus = "completed"
	ScheduledTripStatusCancelled  ScheduledTripStatus = "cancelled"
)

// ScheduledTrip represents a specific trip instance generated from a schedule
type ScheduledTrip struct {
	ID                   string              `json:"id" db:"id"`
	TripScheduleID       string              `json:"trip_schedule_id" db:"trip_schedule_id"`
	PermitID             string              `json:"permit_id" db:"permit_id"`
	BusID                *string             `json:"bus_id,omitempty" db:"bus_id"`
	TripDate             time.Time           `json:"trip_date" db:"trip_date"`
	DepartureTime        string              `json:"departure_time" db:"departure_time"`
	EstimatedArrivalTime *string             `json:"estimated_arrival_time,omitempty" db:"estimated_arrival_time"`
	AssignedDriverID     *string             `json:"assigned_driver_id,omitempty" db:"assigned_driver_id"`
	AssignedConductorID  *string             `json:"assigned_conductor_id,omitempty" db:"assigned_conductor_id"`
	IsBookable           bool                `json:"is_bookable" db:"is_bookable"`
	TotalSeats           int                 `json:"total_seats" db:"total_seats"`
	AvailableSeats       int                 `json:"available_seats" db:"available_seats"`
	BookedSeats          int                 `json:"booked_seats" db:"booked_seats"`
	BaseFare             float64             `json:"base_fare" db:"base_fare"`
	Status               ScheduledTripStatus `json:"status" db:"status"`
	CancellationReason   *string             `json:"cancellation_reason,omitempty" db:"cancellation_reason"`
	CancelledAt          *time.Time          `json:"cancelled_at,omitempty" db:"cancelled_at"`
	SelectedStopIDs      UUIDArray           `json:"selected_stop_ids,omitempty" db:"selected_stop_ids"`
	CreatedAt            time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at" db:"updated_at"`
}

// CreateScheduledTripRequest represents the request to manually create a scheduled trip
type CreateScheduledTripRequest struct {
	TripScheduleID      string  `json:"trip_schedule_id" binding:"required"`
	BusID               *string `json:"bus_id,omitempty"`
	TripDate            string  `json:"trip_date" binding:"required"`
	AssignedDriverID    *string `json:"assigned_driver_id,omitempty"`
	AssignedConductorID *string `json:"assigned_conductor_id,omitempty"`
}

// UpdateScheduledTripRequest represents the request to update a scheduled trip
type UpdateScheduledTripRequest struct {
	BusID               *string `json:"bus_id,omitempty"`
	AssignedDriverID    *string `json:"assigned_driver_id,omitempty"`
	AssignedConductorID *string `json:"assigned_conductor_id,omitempty"`
	Status              *string `json:"status,omitempty"`
	CancellationReason  *string `json:"cancellation_reason,omitempty"`
}

// Validate validates the create scheduled trip request
func (r *CreateScheduledTripRequest) Validate() error {
	// Validate trip date
	if _, err := time.Parse("2006-01-02", r.TripDate); err != nil {
		return errors.New("trip_date must be in YYYY-MM-DD format")
	}

	return nil
}

// CanBeCancelled checks if the trip can be cancelled
func (s *ScheduledTrip) CanBeCancelled() bool {
	return s.Status == ScheduledTripStatusScheduled || s.Status == ScheduledTripStatusConfirmed
}

// IsPastDeparture checks if the trip departure time has passed
func (s *ScheduledTrip) IsPastDeparture() bool {
	now := time.Now()

	// Combine trip date with departure time
	departureTime, err := time.Parse("15:04:05", s.DepartureTime)
	if err != nil {
		// Try without seconds
		departureTime, err = time.Parse("15:04", s.DepartureTime)
		if err != nil {
			return false
		}
	}

	tripDateTime := time.Date(
		s.TripDate.Year(),
		s.TripDate.Month(),
		s.TripDate.Day(),
		departureTime.Hour(),
		departureTime.Minute(),
		departureTime.Second(),
		0,
		s.TripDate.Location(),
	)

	return now.After(tripDateTime)
}

// CanAcceptBooking checks if the trip can accept new bookings
func (s *ScheduledTrip) CanAcceptBooking(seats int) bool {
	if !s.IsBookable {
		return false
	}

	if s.Status != ScheduledTripStatusScheduled && s.Status != ScheduledTripStatusConfirmed {
		return false
	}

	if s.IsPastDeparture() {
		return false
	}

	return s.AvailableSeats >= seats
}

// ReserveSeats reserves seats for a booking
func (s *ScheduledTrip) ReserveSeats(seats int) error {
	if !s.CanAcceptBooking(seats) {
		return errors.New("trip cannot accept bookings")
	}

	s.BookedSeats += seats
	s.AvailableSeats -= seats

	return nil
}

// ReleaseSeats releases seats from a cancelled booking
func (s *ScheduledTrip) ReleaseSeats(seats int) {
	s.BookedSeats -= seats
	s.AvailableSeats += seats

	// Ensure values don't go negative or exceed total
	if s.BookedSeats < 0 {
		s.BookedSeats = 0
	}
	if s.AvailableSeats > s.TotalSeats {
		s.AvailableSeats = s.TotalSeats
	}
}

// OccupancyPercentage returns the percentage of booked seats
func (s *ScheduledTrip) OccupancyPercentage() float64 {
	if s.TotalSeats == 0 {
		return 0
	}
	return float64(s.BookedSeats) / float64(s.TotalSeats) * 100
}
