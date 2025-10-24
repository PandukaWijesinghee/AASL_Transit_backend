package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

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
	return pq.Array(a).Scan(src)
}

// RoutePermit represents a government-issued route permit for a bus owner
type RoutePermit struct {
	ID                       string             `json:"id" db:"id"`
	BusOwnerID               string             `json:"bus_owner_id" db:"bus_owner_id"`
	PermitNumber             string             `json:"permit_number" db:"permit_number"`
	BusRegistrationNumber    string             `json:"bus_registration_number" db:"bus_registration_number"`
	MasterRouteID            *string            `json:"master_route_id,omitempty" db:"master_route_id"`
	RouteNumber              string             `json:"route_number" db:"route_number"`
	RouteName                string             `json:"route_name" db:"route_name"`
	FullOriginCity           string             `json:"full_origin_city" db:"full_origin_city"`
	FullDestinationCity      string             `json:"full_destination_city" db:"full_destination_city"`
	Via                      StringArray        `json:"via,omitempty" db:"via"`
	TotalDistanceKm          *float64           `json:"total_distance_km,omitempty" db:"total_distance_km"`
	EstimatedDurationMinutes *int               `json:"estimated_duration_minutes,omitempty" db:"estimated_duration_minutes"`
	IssueDate                time.Time          `json:"issue_date" db:"issue_date"`
	ExpiryDate               time.Time          `json:"expiry_date" db:"expiry_date"`
	PermitType               string             `json:"permit_type" db:"permit_type"`
	ApprovedFare             float64            `json:"approved_fare" db:"approved_fare"`
	MaxTripsPerDay           *int               `json:"max_trips_per_day,omitempty" db:"max_trips_per_day"`
	AllowedBusTypes          StringArray        `json:"allowed_bus_types,omitempty" db:"allowed_bus_types"`
	Restrictions             *string            `json:"restrictions,omitempty" db:"restrictions"`
	Status                   VerificationStatus `json:"status" db:"status"`
	VerifiedAt               *time.Time         `json:"verified_at,omitempty" db:"verified_at"`
	PermitDocumentURL        *string            `json:"permit_document_url,omitempty" db:"permit_document_url"`
	CreatedAt                time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time          `json:"updated_at" db:"updated_at"`
}

// IsValid checks if the permit is currently valid
func (p *RoutePermit) IsValid() bool {
	now := time.Now()
	return p.Status == VerificationVerified &&
		now.After(p.IssueDate) &&
		now.Before(p.ExpiryDate)
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
func (p *RoutePermit) RouteDisplayName() string {
	return p.RouteNumber + ": " + p.FullOriginCity + " - " + p.FullDestinationCity
}

// CreateRoutePermitRequest represents the request body for creating a permit
type CreateRoutePermitRequest struct {
	PermitNumber          string   `json:"permit_number" binding:"required"`
	BusRegistrationNumber string   `json:"bus_registration_number" binding:"required"`
	RouteNumber           string   `json:"route_number" binding:"required"`
	FromCity              string   `json:"from_city" binding:"required"`
	ToCity                string   `json:"to_city" binding:"required"`
	Via                   *string  `json:"via,omitempty"`
	ApprovedFare          float64  `json:"approved_fare" binding:"required,gt=0"`
	ValidityFrom          string   `json:"validity_from" binding:"required"` // Date format: YYYY-MM-DD
	ValidityTo            string   `json:"validity_to" binding:"required"`   // Date format: YYYY-MM-DD
	TotalDistanceKm       *float64 `json:"total_distance_km,omitempty"`
	EstimatedDuration     *int     `json:"estimated_duration_minutes,omitempty"`
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
	if r.RouteNumber == "" {
		return errors.New("route_number is required")
	}
	if r.FromCity == "" {
		return errors.New("from_city is required")
	}
	if r.ToCity == "" {
		return errors.New("to_city is required")
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
	TotalDistanceKm       *float64 `json:"total_distance_km,omitempty"`
	EstimatedDuration     *int     `json:"estimated_duration_minutes,omitempty"`
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
