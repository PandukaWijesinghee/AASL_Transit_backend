package models

import (
	"database/sql/driver"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// StringArray is a custom type for handling TEXT[] arrays in PostgreSQL
type StringArray []string

// Value implements the driver.Valuer interface
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return pq.Array(a).Value()
}

// Scan implements the sql.Scanner interface
func (a *StringArray) Scan(src interface{}) error {
	// Handle NULL arrays from database
	if src == nil {
		*a = nil
		return nil
	}
	// Convert *StringArray to *[]string so pq.Array can scan properly
	// This is necessary because StringArray is a named type, not []string directly
	slice := (*[]string)(a)
	return pq.Array(slice).Scan(src)
}

// RoutePermit represents a government-issued route permit for a bus owner
// Route details (number, origin, destination, etc.) are stored in master_routes table
type RoutePermit struct {
	ID                    string             `json:"id" db:"id"`
	BusOwnerID            string             `json:"bus_owner_id" db:"bus_owner_id"`
	PermitNumber            string             `json:"permit_number" db:"permit_number"`
	BusRegistrationNumber   string             `json:"bus_registration_number" db:"bus_registration_number"`
	MasterRouteID           string             `json:"master_route_id" db:"master_route_id"` // FK to master_routes
	Via                     StringArray        `json:"via,omitempty" db:"via"`               // Permit-specific stops
	IssueDate               time.Time          `json:"issue_date" db:"issue_date"`
	ExpiryDate              time.Time          `json:"expiry_date" db:"expiry_date"`
	PermitType              string             `json:"permit_type" db:"permit_type"`
	ApprovedFare            float64            `json:"approved_fare" db:"approved_fare"`
	ApprovedSeatingCapacity *int               `json:"approved_seating_capacity,omitempty" db:"approved_seating_capacity"`
	MaxTripsPerDay          *int               `json:"max_trips_per_day,omitempty" db:"max_trips_per_day"`
	AllowedBusTypes       StringArray        `json:"allowed_bus_types,omitempty" db:"allowed_bus_types"`
	Restrictions          *string            `json:"restrictions,omitempty" db:"restrictions"`
	Status                VerificationStatus `json:"status" db:"status"`
	VerifiedAt            *time.Time         `json:"verified_at,omitempty" db:"verified_at"`
	PermitDocumentURL     *string            `json:"permit_document_url,omitempty" db:"permit_document_url"`
	CreatedAt             time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at" db:"updated_at"`
}

// RoutePermitWithDetails includes route information from master_routes (for API responses)
type RoutePermitWithDetails struct {
	RoutePermit
	RouteNumber              string   `json:"route_number" db:"route_number"`
	RouteName                string   `json:"route_name" db:"route_name"`
	FullOriginCity           string   `json:"full_origin_city" db:"origin_city"`
	FullDestinationCity      string   `json:"full_destination_city" db:"destination_city"`
	TotalDistanceKm          *float64 `json:"total_distance_km,omitempty" db:"total_distance_km"`
	EstimatedDurationMinutes *int     `json:"estimated_duration_minutes,omitempty" db:"estimated_duration_minutes"`
	EncodedPolyline          *string  `json:"encoded_polyline,omitempty" db:"encoded_polyline"`
}

// IsValid checks if the permit is currently valid
func (p *RoutePermit) IsValid() bool {
	now := time.Now()
	return p.Status == VerificationVerified &&
		now.After(p.IssueDate) &&
		!now.After(p.ExpiryDate)
}

// IsExpiringSoon checks if the permit is expiring within 30 days
func (p *RoutePermit) IsExpiringSoon() bool {
	now := time.Now()
	daysUntilExpiry := int(p.ExpiryDate.Sub(now).Hours() / 24)
	return daysUntilExpiry <= 30 && daysUntilExpiry > 0
}

// DaysUntilExpiry returns the number of days until the permit expires
func (p *RoutePermit) DaysUntilExpiry() int {
	now := time.Now()
	return int(p.ExpiryDate.Sub(now).Hours() / 24)
}

// RouteDisplayName returns a formatted route display name
// Note: This method is only available on RoutePermitWithDetails (which has route info from JOIN)
func (p *RoutePermitWithDetails) RouteDisplayName() string {
	return p.RouteNumber + ": " + p.FullOriginCity + " - " + p.FullDestinationCity
}

// CreateRoutePermitRequest represents the request body for creating a permit
type CreateRoutePermitRequest struct {
	// Core required fields
	PermitNumber          string   `json:"permit_number" binding:"required"`
	BusRegistrationNumber string   `json:"bus_registration_number" binding:"required"`
	MasterRouteID         string   `json:"master_route_id" binding:"required"` // REQUIRED: Must select from master routes
	ApprovedFare          float64  `json:"approved_fare" binding:"required,gt=0"`
	ValidityFrom          string   `json:"validity_from" binding:"required"` // Date format: YYYY-MM-DD
	ValidityTo            string   `json:"validity_to" binding:"required"`   // Date format: YYYY-MM-DD

	// Optional: Permit-specific intermediate stops (can differ from master route)
	Via                   *string  `json:"via,omitempty"`

	// Optional permit details
	PermitType            *string  `json:"permit_type,omitempty"`
	MaxTripsPerDay        *int     `json:"max_trips_per_day,omitempty"`
	AllowedBusTypes       []string `json:"allowed_bus_types,omitempty"`
	Restrictions          *string  `json:"restrictions,omitempty"`
}

// Validate validates the create permit request
func (r *CreateRoutePermitRequest) Validate() error {
	if r.PermitNumber == "" {
		return errors.New("permit_number is required")
	}
	if r.BusRegistrationNumber == "" {
		return errors.New("bus_registration_number is required")
	}
	if r.MasterRouteID == "" {
		return errors.New("master_route_id is required - please select a route from the dropdown")
	}

	if r.ApprovedFare <= 0 {
		return errors.New("approved_fare must be greater than 0")
	}
	if r.ValidityFrom == "" {
		return errors.New("validity_from is required")
	}
	if r.ValidityTo == "" {
		return errors.New("validity_to is required")
	}

	// Parse dates to ensure they're valid
	issueDate, err := time.Parse("2006-01-02", r.ValidityFrom)
	if err != nil {
		return errors.New("validity_from must be in YYYY-MM-DD format")
	}
	expiryDate, err := time.Parse("2006-01-02", r.ValidityTo)
	if err != nil {
		return errors.New("validity_to must be in YYYY-MM-DD format")
	}

	if expiryDate.Before(issueDate) {
		return errors.New("validity_to must be after validity_from")
	}

	return nil
}

// UpdateRoutePermitRequest represents the request body for updating a permit
type UpdateRoutePermitRequest struct {
	BusRegistrationNumber *string  `json:"bus_registration_number,omitempty"`
	Via                   *string  `json:"via,omitempty"`
	ApprovedFare          *float64 `json:"approved_fare,omitempty"`
	ValidityTo            *string  `json:"validity_to,omitempty"`
	MaxTripsPerDay        *int     `json:"max_trips_per_day,omitempty"`
	AllowedBusTypes       []string `json:"allowed_bus_types,omitempty"`
	Restrictions          *string  `json:"restrictions,omitempty"`
}

// RoutePermitStop represents a stop on a route permit
type RoutePermitStop struct {
	ID                     string     `json:"id" db:"id"`
	RoutePermitID          string     `json:"route_permit_id" db:"route_permit_id"`
	StopName               string     `json:"stop_name" db:"stop_name"`
	StopOrder              int        `json:"stop_order" db:"stop_order"`
	Latitude               *float64   `json:"latitude,omitempty" db:"latitude"`
	Longitude              *float64   `json:"longitude,omitempty" db:"longitude"`
	ArrivalTimeOffsetMins  *int       `json:"arrival_time_offset_minutes,omitempty" db:"arrival_time_offset_minutes"`
	IsMajorStop            bool       `json:"is_major_stop" db:"is_major_stop"`
	CreatedAt              time.Time  `json:"created_at" db:"created_at"`
}

// NewRoutePermitFromRequest creates a RoutePermit from a CreateRoutePermitRequest
func NewRoutePermitFromRequest(busOwnerID string, req *CreateRoutePermitRequest) (*RoutePermit, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Parse dates
	issueDate, err := time.Parse("2006-01-02", req.ValidityFrom)
	if err != nil {
		return nil, errors.New("invalid validity_from date format")
	}

	expiryDate, err := time.Parse("2006-01-02", req.ValidityTo)
	if err != nil {
		return nil, errors.New("invalid validity_to date format")
	}

	// Parse via string to array (permit-specific intermediate stops)
	var via StringArray
	if req.Via != nil && *req.Via != "" {
		parts := make([]string, 0)
		for _, part := range splitAndTrim(*req.Via, ",") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		via = StringArray(parts)
	}

	// Default permit type
	permitType := "regular"
	if req.PermitType != nil && *req.PermitType != "" {
		permitType = *req.PermitType
	}

	// Convert allowed bus types
	var allowedBusTypes StringArray
	if len(req.AllowedBusTypes) > 0 {
		allowedBusTypes = StringArray(req.AllowedBusTypes)
	}

	// Create permit with only master_route_id (route details come from JOIN)
	return &RoutePermit{
		ID:                    uuid.New().String(),
		BusOwnerID:            busOwnerID,
		PermitNumber:          req.PermitNumber,
		BusRegistrationNumber: req.BusRegistrationNumber,
		MasterRouteID:         req.MasterRouteID, // FK to master_routes
		Via:                   via,                // Permit-specific stops (optional)
		ApprovedFare:          req.ApprovedFare,
		IssueDate:             issueDate,
		ExpiryDate:            expiryDate,
		PermitType:            permitType,
		MaxTripsPerDay:        req.MaxTripsPerDay,
		AllowedBusTypes:       allowedBusTypes,
		Restrictions:          req.Restrictions,
		Status:                VerificationPending,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}, nil
}

// Helper function to split string by delimiter and trim spaces
func splitAndTrim(s string, delimiter string) []string {
	parts := make([]string, 0)
	for _, part := range split(s, delimiter) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// Helper to split string
func split(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}
