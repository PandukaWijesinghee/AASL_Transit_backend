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

// ScheduledTrip represents a specific trip instance generated from a schedule or created as a special trip
type ScheduledTrip struct {
	ID                       string              `json:"id" db:"id"`
	TripScheduleID           *string             `json:"trip_schedule_id,omitempty" db:"trip_schedule_id"`     // Nullable for special trips
	BusOwnerRouteID          *string             `json:"bus_owner_route_id,omitempty" db:"bus_owner_route_id"` // Optional route override - inherits from schedule if NULL
	PermitID                 *string             `json:"permit_id,omitempty" db:"permit_id"`                   // Nullable - assigned later
	BusID                    *string             `json:"bus_id,omitempty" db:"bus_id"`
	DepartureDatetime        time.Time           `json:"departure_datetime" db:"departure_datetime"`                           // Specific departure date and time (e.g., 2025-11-20 22:00:00)
	EstimatedDurationMinutes *int                `json:"estimated_duration_minutes,omitempty" db:"estimated_duration_minutes"` // Duration in minutes (industry standard - calculate arrival on-the-fly)
	AssignedDriverID         *string             `json:"assigned_driver_id,omitempty" db:"assigned_driver_id"`
	AssignedConductorID      *string             `json:"assigned_conductor_id,omitempty" db:"assigned_conductor_id"`
	SeatLayoutID             *string             `json:"seat_layout_id,omitempty" db:"seat_layout_id"` // Required before trip can be published for booking
	IsBookable               bool                `json:"is_bookable" db:"is_bookable"`                 // Controls if trip is available for passenger booking
	EverPublished            bool                `json:"ever_published" db:"ever_published"`           // Tracks if trip was ever made bookable (stays true once set)
	TotalSeats               int                 `json:"total_seats" db:"total_seats"`
	AvailableSeats           int                 `json:"available_seats" db:"available_seats"`
	BookedSeats              int                 `json:"booked_seats" db:"booked_seats"`
	BaseFare                 float64             `json:"base_fare" db:"base_fare"`
	BookingAdvanceHours      int                 `json:"booking_advance_hours" db:"booking_advance_hours"`       // NEW: Hours before trip that booking opens
	AssignmentDeadline       *time.Time          `json:"assignment_deadline,omitempty" db:"assignment_deadline"` // NEW: Deadline to assign resources
	Status                   ScheduledTripStatus `json:"status" db:"status"`
	CancellationReason       *string             `json:"cancellation_reason,omitempty" db:"cancellation_reason"`
	CancelledAt              *time.Time          `json:"cancelled_at,omitempty" db:"cancelled_at"`
	SelectedStopIDs          UUIDArray           `json:"selected_stop_ids,omitempty" db:"selected_stop_ids"`
	CreatedAt                time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time           `json:"updated_at" db:"updated_at"`
}

// GetArrivalDatetime calculates arrival datetime (industry standard approach)
// Always calculates from departure + duration (no stored arrival time)
func (t *ScheduledTrip) GetArrivalDatetime() *time.Time {
	if t.EstimatedDurationMinutes == nil {
		return nil // No duration = can't calculate arrival
	}
	arrival := t.DepartureDatetime.Add(time.Duration(*t.EstimatedDurationMinutes) * time.Minute)
	return &arrival
}

// IsOvernight checks if trip crosses midnight
func (t *ScheduledTrip) IsOvernight() bool {
	arrival := t.GetArrivalDatetime()
	if arrival == nil {
		return false
	}
	return arrival.Day() != t.DepartureDatetime.Day() ||
		arrival.Month() != t.DepartureDatetime.Month() ||
		arrival.Year() != t.DepartureDatetime.Year()
}

// CreateScheduledTripRequest represents the request to manually create a scheduled trip
type CreateScheduledTripRequest struct {
	TripScheduleID      string  `json:"trip_schedule_id" binding:"required"`
	BusID               *string `json:"bus_id,omitempty"`
	DepartureDatetime   string  `json:"departure_datetime" binding:"required"` // ISO 8601 datetime
	AssignedDriverID    *string `json:"assigned_driver_id,omitempty"`
	AssignedConductorID *string `json:"assigned_conductor_id,omitempty"`
}

// CreateSpecialTripRequest represents the request to create a special one-time trip (no timetable)
type CreateSpecialTripRequest struct {
	CustomRouteID            string  `json:"custom_route_id" binding:"required"`
	PermitID                 *string `json:"permit_id,omitempty"`                   // Changed to pointer for nullable
	DepartureDatetime        string  `json:"departure_datetime" binding:"required"` // ISO 8601 datetime: 2025-11-20T22:00:00Z or 2025-11-20 22:00:00
	EstimatedDurationMinutes *int    `json:"estimated_duration_minutes,omitempty"`  // Duration in minutes (optional, for calculating arrival)
	BaseFare                 float64 `json:"base_fare" binding:"required,gt=0"`
	MaxBookableSeats         int     `json:"max_bookable_seats" binding:"required,gt=0"`
	IsBookable               bool    `json:"is_bookable"`
	BookingAdvanceHours      *int    `json:"booking_advance_hours,omitempty"` // NULL = use system default (72)
	// Resource assignment (required if trip is soon)
	BusID               *string `json:"bus_id,omitempty"`
	AssignedDriverID    *string `json:"assigned_driver_id,omitempty"`
	AssignedConductorID *string `json:"assigned_conductor_id,omitempty"`
}

// Validate validates the create special trip request
func (r *CreateSpecialTripRequest) Validate() error {
	// Validate departure_datetime format (supports multiple formats)
	var departureDatetime time.Time
	var err error

	// Try ISO 8601 with timezone
	departureDatetime, err = time.Parse(time.RFC3339, r.DepartureDatetime)
	if err != nil {
		// Try common datetime format without timezone
		departureDatetime, err = time.Parse("2006-01-02 15:04:05", r.DepartureDatetime)
		if err != nil {
			// Try with T separator
			departureDatetime, err = time.Parse("2006-01-02T15:04:05", r.DepartureDatetime)
			if err != nil {
				return errors.New("departure_datetime must be in ISO 8601 format (e.g., 2025-11-20T22:00:00Z or 2025-11-20 22:00:00)")
			}
		}
	}

	// Validate departure datetime is not in the past
	if departureDatetime.Before(time.Now()) {
		return errors.New("departure_datetime cannot be in the past")
	}

	// Validate duration if provided
	if r.EstimatedDurationMinutes != nil {
		if *r.EstimatedDurationMinutes <= 0 {
			return errors.New("estimated_duration_minutes must be greater than 0")
		}
		if *r.EstimatedDurationMinutes > 2880 {
			return errors.New("estimated_duration_minutes cannot exceed 2880 minutes (48 hours)")
		}
	}

	// Validate booking_advance_hours if provided (must be >= 72)
	if r.BookingAdvanceHours != nil && *r.BookingAdvanceHours < 72 {
		return errors.New("booking_advance_hours must be >= 72 (system minimum)")
	}

	return nil
}

// UpdateScheduledTripRequest represents the request to update a scheduled trip
type UpdateScheduledTripRequest struct {
	BusOwnerRouteID     *string `json:"bus_owner_route_id,omitempty"` // Optional route override
	BusID               *string `json:"bus_id,omitempty"`
	AssignedDriverID    *string `json:"assigned_driver_id,omitempty"`
	AssignedConductorID *string `json:"assigned_conductor_id,omitempty"`
	Status              *string `json:"status,omitempty"`
	CancellationReason  *string `json:"cancellation_reason,omitempty"`
}

// Validate validates the create scheduled trip request
func (r *CreateScheduledTripRequest) Validate() error {
	// Validate departure_datetime format
	var err error
	_, err = time.Parse(time.RFC3339, r.DepartureDatetime)
	if err != nil {
		_, err = time.Parse("2006-01-02 15:04:05", r.DepartureDatetime)
		if err != nil {
			_, err = time.Parse("2006-01-02T15:04:05", r.DepartureDatetime)
			if err != nil {
				return errors.New("departure_datetime must be in ISO 8601 format")
			}
		}
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
	return now.After(s.DepartureDatetime)
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

// ScheduledTripWithRouteInfo extends ScheduledTrip with route details
type ScheduledTripWithRouteInfo struct {
	ScheduledTrip
	RouteNumber     *string `json:"route_number,omitempty"`
	OriginCity      *string `json:"origin_city,omitempty"`
	DestinationCity *string `json:"destination_city,omitempty"`
	IsUpDirection   *bool   `json:"is_up_direction,omitempty"`
}

// StaffDetails contains basic staff information for trip display
type StaffDetails struct {
	ID            string  `json:"id"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	Phone         string  `json:"phone"`
	LicenseNumber *string `json:"license_number,omitempty"`
}

// PermitDetails contains basic permit information for trip display
type PermitDetails struct {
	ID                    string `json:"id"`
	PermitNumber          string `json:"permit_number"`
	BusRegistrationNumber string `json:"bus_registration_number"`
	RouteNumber           string `json:"route_number"`
	OriginCity            string `json:"origin_city"`
	DestinationCity       string `json:"destination_city"`
}

// ScheduledTripWithDetails extends ScheduledTrip with full assignment details
type ScheduledTripWithDetails struct {
	ScheduledTrip
	Driver    *StaffDetails  `json:"driver,omitempty"`
	Conductor *StaffDetails  `json:"conductor,omitempty"`
	Permit    *PermitDetails `json:"permit,omitempty"`
}
