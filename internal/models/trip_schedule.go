package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RecurrenceType represents how often a trip schedule repeats
type RecurrenceType string

const (
	RecurrenceDaily    RecurrenceType = "daily"
	RecurrenceWeekly   RecurrenceType = "weekly"
	RecurrenceInterval RecurrenceType = "interval"
	// Deprecated: RecurrenceSpecificDates is no longer used in timetable system
	RecurrenceSpecificDates RecurrenceType = "specific_dates"
)

// TripSchedule represents a recurring trip template (timetable)
type TripSchedule struct {
	ID                   string         `json:"id" db:"id"`
	BusOwnerID           string         `json:"bus_owner_id" db:"bus_owner_id"`
	PermitID             *string        `json:"permit_id,omitempty" db:"permit_id"`                   // Optional - assigned later to specific trips
	BusOwnerRouteID      *string        `json:"bus_owner_route_id,omitempty" db:"bus_owner_route_id"` // Reference to bus_owner_routes (replaces old custom_route_id)
	BusID                *string        `json:"bus_id,omitempty" db:"bus_id"`
	ScheduleName         *string        `json:"schedule_name,omitempty" db:"schedule_name"`
	RecurrenceType       RecurrenceType `json:"recurrence_type" db:"recurrence_type"`
	RecurrenceDays       string         `json:"recurrence_days,omitempty" db:"recurrence_days"`               // For weekly: "1,2,3" (comma-separated day numbers)
	RecurrenceInterval   *int           `json:"recurrence_interval,omitempty" db:"recurrence_interval"`       // NEW: For interval: every N days
	DepartureTime        string         `json:"departure_time" db:"departure_time"`                           // TIME type stored as string (HH:MM:SS)
	EstimatedArrivalTime *string        `json:"estimated_arrival_time,omitempty" db:"estimated_arrival_time"` // TIME type stored as string (HH:MM:SS)
	BaseFare             float64        `json:"base_fare" db:"base_fare"`
	IsBookable           bool           `json:"is_bookable" db:"is_bookable"`
	MaxBookableSeats     *int           `json:"max_bookable_seats,omitempty" db:"max_bookable_seats"`
	BookingAdvanceHours  *int           `json:"booking_advance_hours,omitempty" db:"booking_advance_hours"` // NEW: NULL = use system default
	IsActive             bool           `json:"is_active" db:"is_active"`
	Notes                *string        `json:"notes,omitempty" db:"notes"`
	CreatedAt            time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at" db:"updated_at"`

	// Deprecated fields (kept for backward compatibility, renamed in DB to *_old)
	Direction           string     `json:"direction,omitempty" db:"direction_old"`
	TripsPerDay         int        `json:"trips_per_day,omitempty" db:"trips_per_day_old"`
	AdvanceBookingHours int        `json:"advance_booking_hours,omitempty" db:"advance_booking_hours"`
	DefaultDriverID     *string    `json:"default_driver_id,omitempty" db:"default_driver_id"`
	DefaultConductorID  *string    `json:"default_conductor_id,omitempty" db:"default_conductor_id"`
	SelectedStopIDs     UUIDArray  `json:"selected_stop_ids,omitempty" db:"selected_stop_ids"`
	ValidFrom           time.Time  `json:"valid_from,omitempty" db:"valid_from_old"`
	ValidUntil          *time.Time `json:"valid_until,omitempty" db:"valid_until_old"`
	SpecificDates       *string    `json:"specific_dates,omitempty" db:"specific_dates"` // Comma-separated dates: "2025-01-01,2025-01-15" - can be NULL
}

// Helper methods for converting between string and slices

// GetRecurrenceDaysSlice parses the comma-separated recurrence_days string into []int
func (s *TripSchedule) GetRecurrenceDaysSlice() ([]int, error) {
	if s.RecurrenceDays == "" {
		return []int{}, nil
	}
	return StringToIntSlice(s.RecurrenceDays)
}

// SetRecurrenceDaysFromSlice converts []int to comma-separated string
func (s *TripSchedule) SetRecurrenceDaysFromSlice(days []int) {
	s.RecurrenceDays = IntSliceToString(days)
}

// GetSpecificDatesSlice parses the comma-separated specific_dates string into []time.Time
func (s *TripSchedule) GetSpecificDatesSlice() ([]time.Time, error) {
	if s.SpecificDates == nil || *s.SpecificDates == "" {
		return []time.Time{}, nil
	}
	return StringToDateSlice(*s.SpecificDates)
}

// SetSpecificDatesFromSlice converts []time.Time to comma-separated string
func (s *TripSchedule) SetSpecificDatesFromSlice(dates []time.Time) {
	dateStr := DateSliceToString(dates)
	s.SpecificDates = &dateStr
}

// Helper functions for string ↔ slice conversion

// IntSliceToString converts []int to comma-separated string (e.g., "1,2,3")
func IntSliceToString(slice []int) string {
	if len(slice) == 0 {
		return ""
	}
	strSlice := make([]string, len(slice))
	for i, v := range slice {
		strSlice[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(strSlice, ",")
}

// StringToIntSlice converts comma-separated string to []int
func StringToIntSlice(str string) ([]int, error) {
	if str == "" {
		return []int{}, nil
	}
	parts := strings.Split(str, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid integer in string: %s", part)
		}
		result = append(result, num)
	}
	return result, nil
}

// DateSliceToString converts []time.Time to comma-separated string (e.g., "2025-01-01,2025-01-15")
func DateSliceToString(dates []time.Time) string {
	if len(dates) == 0 {
		return ""
	}
	strSlice := make([]string, len(dates))
	for i, d := range dates {
		strSlice[i] = d.Format("2006-01-02")
	}
	return strings.Join(strSlice, ",")
}

// StringToDateSlice converts comma-separated string to []time.Time
func StringToDateSlice(str string) ([]time.Time, error) {
	if str == "" {
		return []time.Time{}, nil
	}
	parts := strings.Split(str, ",")
	result := make([]time.Time, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", part)
		if err != nil {
			return nil, fmt.Errorf("invalid date in string: %s", part)
		}
		result = append(result, date)
	}
	return result, nil
}

// CreateTimetableRequest represents the request to create a new timetable (trip schedule)
type CreateTimetableRequest struct {
	CustomRouteID        string  `json:"custom_route_id" binding:"required"`
	PermitID             *string `json:"permit_id,omitempty"` // Optional - assigned later to specific trips
	ScheduleName         *string `json:"schedule_name,omitempty"`
	DepartureTime        string  `json:"departure_time" binding:"required"`
	EstimatedArrivalTime *string `json:"estimated_arrival_time,omitempty"`
	BaseFare             float64 `json:"base_fare" binding:"required,gt=0"`
	MaxBookableSeats     int     `json:"max_bookable_seats" binding:"required,gt=0"`
	IsBookable           bool    `json:"is_bookable"`
	BookingAdvanceHours  *int    `json:"booking_advance_hours,omitempty"` // NULL = use system default (72h)
	RecurrenceType       string  `json:"recurrence_type" binding:"required,oneof=daily weekly interval"`
	RecurrenceDays       []int   `json:"recurrence_days,omitempty"`     // Required for weekly
	RecurrenceInterval   *int    `json:"recurrence_interval,omitempty"` // Required for interval
	ValidFrom            string  `json:"valid_from" binding:"required"` // NEW: Timetable validity start date
	ValidUntil           *string `json:"valid_until,omitempty"`         // NEW: Optional end date
	Notes                *string `json:"notes,omitempty"`
}

// Validate validates the create timetable request
func (r *CreateTimetableRequest) Validate() error {
	// Validate recurrence type specific requirements
	switch RecurrenceType(r.RecurrenceType) {
	case RecurrenceWeekly:
		if len(r.RecurrenceDays) == 0 {
			return errors.New("recurrence_days is required for weekly schedules")
		}
		// Validate days are 0-6 (Sunday-Saturday)
		for _, day := range r.RecurrenceDays {
			if day < 0 || day > 6 {
				return errors.New("recurrence_days must contain values between 0 (Sunday) and 6 (Saturday)")
			}
		}
	case RecurrenceInterval:
		if r.RecurrenceInterval == nil || *r.RecurrenceInterval < 2 {
			return errors.New("recurrence_interval is required for interval schedules and must be >= 2")
		}
	}

	// Validate departure time format (HH:MM or HH:MM:SS)
	if _, err := time.Parse("15:04", r.DepartureTime); err != nil {
		if _, err := time.Parse("15:04:05", r.DepartureTime); err != nil {
			return errors.New("departure_time must be in HH:MM or HH:MM:SS format")
		}
	}

	// Validate estimated arrival time format if provided
	if r.EstimatedArrivalTime != nil {
		if _, err := time.Parse("15:04", *r.EstimatedArrivalTime); err != nil {
			if _, err := time.Parse("15:04:05", *r.EstimatedArrivalTime); err != nil {
				return errors.New("estimated_arrival_time must be in HH:MM or HH:MM:SS format")
			}
		}
	}

	// Validate valid_from date format (YYYY-MM-DD)
	validFrom, err := time.Parse("2006-01-02", r.ValidFrom)
	if err != nil {
		return errors.New("valid_from must be in YYYY-MM-DD format")
	}

	// Validate valid_until date format if provided
	if r.ValidUntil != nil {
		validUntil, err := time.Parse("2006-01-02", *r.ValidUntil)
		if err != nil {
			return errors.New("valid_until must be in YYYY-MM-DD format")
		}
		// valid_until must be after valid_from
		if validUntil.Before(validFrom) {
			return errors.New("valid_until must be after valid_from")
		}
	}

	// Validate booking_advance_hours if provided (must be >= 72)
	if r.BookingAdvanceHours != nil && *r.BookingAdvanceHours < 72 {
		return errors.New("booking_advance_hours must be >= 72 (system minimum)")
	}

	return nil
}

// Deprecated: CreateTripScheduleRequest - use CreateTimetableRequest instead
type CreateTripScheduleRequest struct {
	PermitID             string   `json:"permit_id" binding:"required"`
	BusID                *string  `json:"bus_id,omitempty"`
	ScheduleName         *string  `json:"schedule_name,omitempty"`
	RecurrenceType       string   `json:"recurrence_type" binding:"required,oneof=daily weekly specific_dates"`
	RecurrenceDays       []int    `json:"recurrence_days,omitempty"`
	SpecificDates        []string `json:"specific_dates,omitempty"` // Date strings in YYYY-MM-DD format
	DepartureTime        string   `json:"departure_time" binding:"required"`
	EstimatedArrivalTime *string  `json:"estimated_arrival_time,omitempty"` // HH:MM or HH:MM:SS format
	Direction            string   `json:"direction" binding:"required,oneof=UP DOWN ROUND_TRIP"`
	TripsPerDay          int      `json:"trips_per_day" binding:"required,min=1,max=10"`
	BaseFare             float64  `json:"base_fare" binding:"required,gt=0"`
	IsBookable           bool     `json:"is_bookable"`
	MaxBookableSeats     *int     `json:"max_bookable_seats,omitempty"`
	AdvanceBookingHours  int      `json:"advance_booking_hours"`
	DefaultDriverID      *string  `json:"default_driver_id,omitempty"`
	DefaultConductorID   *string  `json:"default_conductor_id,omitempty"`
	SelectedStopIDs      []string `json:"selected_stop_ids,omitempty"`
	ValidFrom            string   `json:"valid_from" binding:"required"`
	ValidUntil           *string  `json:"valid_until,omitempty"`
	Notes                *string  `json:"notes,omitempty"`
}

// Validate validates the create trip schedule request
func (r *CreateTripScheduleRequest) Validate() error {
	// Check recurrence type specific validations
	switch RecurrenceType(r.RecurrenceType) {
	case RecurrenceWeekly:
		if len(r.RecurrenceDays) == 0 {
			return errors.New("recurrence_days is required for weekly schedules")
		}
		// Validate days are 0-6 (Sunday-Saturday)
		for _, day := range r.RecurrenceDays {
			if day < 0 || day > 6 {
				return errors.New("recurrence_days must contain values between 0 (Sunday) and 6 (Saturday)")
			}
		}
	case RecurrenceSpecificDates:
		if len(r.SpecificDates) == 0 {
			return errors.New("specific_dates is required for specific_dates recurrence type")
		}
	}

	// Validate departure time format (HH:MM or HH:MM:SS)
	if _, err := time.Parse("15:04", r.DepartureTime); err != nil {
		if _, err := time.Parse("15:04:05", r.DepartureTime); err != nil {
			return errors.New("departure_time must be in HH:MM or HH:MM:SS format")
		}
	}

	// Validate dates
	validFrom, err := time.Parse("2006-01-02", r.ValidFrom)
	if err != nil {
		return errors.New("valid_from must be in YYYY-MM-DD format")
	}

	if r.ValidUntil != nil {
		validUntil, err := time.Parse("2006-01-02", *r.ValidUntil)
		if err != nil {
			return errors.New("valid_until must be in YYYY-MM-DD format")
		}
		if validUntil.Before(validFrom) {
			return errors.New("valid_until must be after valid_from")
		}
	}

	// Validate booking settings
	if r.IsBookable && r.MaxBookableSeats != nil && *r.MaxBookableSeats <= 0 {
		return errors.New("max_bookable_seats must be greater than 0 if specified")
	}

	return nil
}

// IsValidForDate checks if the schedule is valid for a specific date
func (s *TripSchedule) IsValidForDate(date time.Time) bool {
	if !s.IsActive {
		return false
	}

	// For backward compatibility: Check if date is within valid range (old system)
	if !s.ValidFrom.IsZero() && date.Before(s.ValidFrom) {
		return false
	}

	if s.ValidUntil != nil && date.After(*s.ValidUntil) {
		return false
	}

	// Check recurrence rules
	switch s.RecurrenceType {
	case RecurrenceDaily:
		return true
	case RecurrenceWeekly:
		weekday := int(date.Weekday())
		days, err := s.GetRecurrenceDaysSlice()
		if err != nil {
			return false
		}
		for _, day := range days {
			if day == weekday {
				return true
			}
		}
		return false
	case RecurrenceInterval:
		// For interval: calculate days from creation date
		if s.RecurrenceInterval == nil || *s.RecurrenceInterval <= 0 {
			return false
		}
		daysDiff := int(date.Sub(s.CreatedAt).Hours() / 24)
		return daysDiff >= 0 && daysDiff%*s.RecurrenceInterval == 0
	case RecurrenceSpecificDates:
		// Deprecated: for backward compatibility only
		dateOnly := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		specificDates, err := s.GetSpecificDatesSlice()
		if err != nil {
			return false
		}
		for _, specificDate := range specificDates {
			specificDateOnly := time.Date(specificDate.Year(), specificDate.Month(), specificDate.Day(), 0, 0, 0, 0, time.UTC)
			if dateOnly.Equal(specificDateOnly) {
				return true
			}
		}
		return false
	}

	return false
}

// GetNextOccurrences returns the next N dates when this schedule will run
func (s *TripSchedule) GetNextOccurrences(n int) []time.Time {
	dates := make([]time.Time, 0, n)
	currentDate := time.Now()

	// Start from valid_from if it's in the future (for backward compatibility)
	if !s.ValidFrom.IsZero() && s.ValidFrom.After(currentDate) {
		currentDate = s.ValidFrom
	}

	// For interval type, we can optimize by directly calculating occurrences
	if s.RecurrenceType == RecurrenceInterval && s.RecurrenceInterval != nil {
		interval := *s.RecurrenceInterval
		// Find the next occurrence from creation date
		daysSinceCreation := int(currentDate.Sub(s.CreatedAt).Hours() / 24)
		daysUntilNext := interval - (daysSinceCreation % interval)
		if daysUntilNext == interval {
			daysUntilNext = 0 // Today is an occurrence
		}

		nextDate := currentDate.AddDate(0, 0, daysUntilNext)
		for i := 0; i < n; i++ {
			dates = append(dates, nextDate)
			nextDate = nextDate.AddDate(0, 0, interval)
		}
		return dates
	}

	// For daily and weekly, iterate through days
	maxDays := 365
	dayCount := 0

	for len(dates) < n && dayCount < maxDays {
		if s.IsValidForDate(currentDate) {
			dates = append(dates, currentDate)
		}
		currentDate = currentDate.AddDate(0, 0, 1)
		dayCount++
	}

	return dates
}
