package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SearchRequest represents a passenger's search query
type SearchRequest struct {
	From     string     `json:"from" binding:"required"` // Origin stop name (e.g., "Colombo Fort")
	To       string     `json:"to" binding:"required"`   // Destination stop name (e.g., "Kandy")
	DateTime *time.Time `json:"datetime,omitempty"`      // Optional: Departure date/time filter
	Limit    int        `json:"limit,omitempty"`         // Optional: Max results (default: 20)
}

// SearchResponse represents the search results returned to passenger
type SearchResponse struct {
	Status        string        `json:"status"`         // "success", "partial", "error"
	Message       string        `json:"message"`        // Human-readable message
	SearchDetails SearchDetails `json:"search_details"` // Details about the search
	Results       []TripResult  `json:"results"`        // List of matching trips
	SearchTimeMs  int64         `json:"search_time_ms"` // Search execution time
}

// SearchDetails provides information about how the search was performed
type SearchDetails struct {
	FromStop   StopInfo `json:"from_stop"`   // Origin stop details
	ToStop     StopInfo `json:"to_stop"`     // Destination stop details
	SearchType string   `json:"search_type"` // "exact", "fuzzy", "failed"
}

// StopInfo represents a bus stop with matching details
type StopInfo struct {
	ID            *uuid.UUID `json:"id,omitempty"`   // Stop ID (NULL if not found)
	Name          string     `json:"name,omitempty"` // Matched stop name
	Matched       bool       `json:"matched"`        // Whether stop was found
	OriginalInput string     `json:"original_input"` // What user typed
}

// RouteStop represents a stop on the route for boarding/alighting selection
type RouteStop struct {
	ID                       string   `json:"id" db:"id"`
	StopName                 string   `json:"stop_name" db:"stop_name"`
	StopOrder                int      `json:"stop_order" db:"stop_order"`
	Latitude                 *float64 `json:"latitude,omitempty" db:"latitude"`
	Longitude                *float64 `json:"longitude,omitempty" db:"longitude"`
	ArrivalTimeOffsetMinutes *int     `json:"arrival_time_offset_minutes,omitempty" db:"arrival_time_offset_minutes"`
	IsMajorStop              bool     `json:"is_major_stop" db:"is_major_stop"`
}

// TripResult represents a single trip in search results
type TripResult struct {
	TripID           uuid.UUID `json:"trip_id" db:"trip_id"`
	RouteName        string    `json:"route_name" db:"route_name"`
	RouteNumber      *string   `json:"route_number,omitempty" db:"route_number"`
	BusType          string    `json:"bus_type" db:"bus_type"`
	DepartureTime    time.Time `json:"-" db:"departure_time"`
	EstimatedArrival time.Time `json:"-" db:"estimated_arrival"`
	DurationMinutes  int       `json:"duration_minutes" db:"duration_minutes"`
	// AvailableSeats removed - will be calculated from booking table in separate query
	TotalSeats    int         `json:"total_seats" db:"total_seats"`
	Fare          float64     `json:"fare" db:"fare"`
	BoardingPoint string      `json:"boarding_point" db:"boarding_point"`
	DroppingPoint string      `json:"dropping_point" db:"dropping_point"`
	BusFeatures   BusFeatures `json:"bus_features"`
	IsBookable    bool        `json:"is_bookable" db:"is_bookable"`
	// Route stops for passenger to select boarding/alighting points
	RouteStops []RouteStop `json:"route_stops,omitempty"`
	// Internal fields for building route stops (not in JSON)
	BusOwnerRouteID *string `json:"-" db:"bus_owner_route_id"`
	MasterRouteID   *string `json:"-" db:"master_route_id"`
}

// MarshalJSON implements custom JSON marshaling to handle timestamps without timezone
func (tr TripResult) MarshalJSON() ([]byte, error) {
	// Load Asia/Colombo timezone (Sri Lanka)
	loc, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		// Fallback to UTC if timezone loading fails
		loc = time.UTC
	}

	// Always convert database timestamps to Asia/Colombo timezone
	// Database stores times without timezone, so we interpret them as Sri Lankan local time
	departureTime := time.Date(
		tr.DepartureTime.Year(), tr.DepartureTime.Month(), tr.DepartureTime.Day(),
		tr.DepartureTime.Hour(), tr.DepartureTime.Minute(), tr.DepartureTime.Second(),
		tr.DepartureTime.Nanosecond(), loc,
	)

	estimatedArrival := time.Date(
		tr.EstimatedArrival.Year(), tr.EstimatedArrival.Month(), tr.EstimatedArrival.Day(),
		tr.EstimatedArrival.Hour(), tr.EstimatedArrival.Minute(), tr.EstimatedArrival.Second(),
		tr.EstimatedArrival.Nanosecond(), loc,
	)

	type Alias TripResult
	return json.Marshal(&struct {
		DepartureTime    string `json:"departure_time"`
		EstimatedArrival string `json:"estimated_arrival"`
		*Alias
	}{
		DepartureTime:    departureTime.Format(time.RFC3339),
		EstimatedArrival: estimatedArrival.Format(time.RFC3339),
		Alias:            (*Alias)(&tr),
	})
}

// BusFeatures represents amenities available on the bus
type BusFeatures struct {
	HasWiFi          bool `json:"has_wifi" db:"has_wifi"`
	HasAC            bool `json:"has_ac" db:"has_ac"`
	HasChargingPorts bool `json:"has_charging_ports" db:"has_charging_ports"`
	HasEntertainment bool `json:"has_entertainment" db:"has_entertainment"`
	HasRefreshments  bool `json:"has_refreshments" db:"has_refreshments"`
}

// PopularRoute represents a frequently searched route for quick selection
type PopularRoute struct {
	FromStopName string `json:"from_stop_name"`
	ToStopName   string `json:"to_stop_name"`
	RouteCount   int    `json:"route_count"`            // Number of routes available
	SearchCount  *int   `json:"search_count,omitempty"` // How many times searched (from analytics)
}

// StopAutocomplete represents a stop suggestion for autocomplete
type StopAutocomplete struct {
	StopID     uuid.UUID `json:"stop_id" db:"stop_id"`
	StopName   string    `json:"stop_name" db:"stop_name"`
	RouteCount int       `json:"route_count" db:"route_count"` // Number of routes serving this stop
}

// SearchLog represents a search analytics record
type SearchLog struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	FromInput      string     `json:"from_input" db:"from_input"`
	ToInput        string     `json:"to_input" db:"to_input"`
	FromStopID     *uuid.UUID `json:"from_stop_id,omitempty" db:"from_stop_id"`
	ToStopID       *uuid.UUID `json:"to_stop_id,omitempty" db:"to_stop_id"`
	ResultsCount   int        `json:"results_count" db:"results_count"`
	ResponseTimeMs int64      `json:"response_time_ms" db:"response_time_ms"`
	UserID         *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	IPAddress      *string    `json:"ip_address,omitempty" db:"ip_address"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

// Validate validates the search request
func (r *SearchRequest) Validate() error {
	if r.From == "" {
		return ErrInvalidInput("from location is required")
	}
	if r.To == "" {
		return ErrInvalidInput("to location is required")
	}
	if r.From == r.To {
		return ErrInvalidInput("origin and destination cannot be the same")
	}

	// Set default limit if not provided
	if r.Limit <= 0 {
		r.Limit = 20
	}
	// Cap maximum limit
	if r.Limit > 100 {
		r.Limit = 100
	}

	return nil
}

// GetSearchDateTime returns the search datetime or current time
func (r *SearchRequest) GetSearchDateTime() time.Time {
	if r.DateTime != nil {
		return *r.DateTime
	}
	return time.Now()
}

// ErrInvalidInput creates a validation error
func ErrInvalidInput(message string) error {
	return &ValidationError{Message: message}
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
