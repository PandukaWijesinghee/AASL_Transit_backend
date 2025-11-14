package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Lounge represents a physical lounge location
type Lounge struct {
	ID            uuid.UUID `db:"id" json:"id"`
	LoungeOwnerID uuid.UUID `db:"lounge_owner_id" json:"lounge_owner_id"`

	// Lounge Information
	LoungeName  string         `db:"lounge_name" json:"lounge_name"`
	Description sql.NullString `db:"description" json:"description,omitempty"`

	// Location
	Address    string         `db:"address" json:"address"`
	State      sql.NullString `db:"state" json:"state,omitempty"`
	Country    sql.NullString `db:"country" json:"country,omitempty"`
	PostalCode sql.NullString `db:"postal_code" json:"postal_code,omitempty"`
	Latitude   sql.NullString `db:"latitude" json:"latitude,omitempty"`   // DECIMAL stored as string
	Longitude  sql.NullString `db:"longitude" json:"longitude,omitempty"` // DECIMAL stored as string

	// Contact
	ContactPhone sql.NullString `db:"contact_phone" json:"contact_phone,omitempty"`

	// Capacity
	Capacity sql.NullInt64 `db:"capacity" json:"capacity,omitempty"` // Maximum number of people

	// Pricing (in LKR)
	Price1Hour    sql.NullString `db:"price_1_hour" json:"price_1_hour,omitempty"`       // DECIMAL stored as string
	Price2Hours   sql.NullString `db:"price_2_hours" json:"price_2_hours,omitempty"`     // DECIMAL stored as string
	Price3Hours   sql.NullString `db:"price_3_hours" json:"price_3_hours,omitempty"`     // DECIMAL stored as string
	PriceUntilBus sql.NullString `db:"price_until_bus" json:"price_until_bus,omitempty"` // DECIMAL stored as string

	// Amenities (JSONB - array of strings)
	Amenities []byte `db:"amenities" json:"amenities,omitempty"` // ["wifi", "ac", "charging"]

	// Images (JSONB - array of URLs)
	Images []byte `db:"images" json:"images,omitempty"` // ["url1", "url2"]

	// Status
	Status        string `db:"status" json:"status"` // pending, active, inactive, suspended
	IsOperational bool   `db:"is_operational" json:"is_operational"`

	// Metadata
	AverageRating sql.NullString `db:"average_rating" json:"average_rating,omitempty"` // DECIMAL stored as string

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// LoungeStatus constants
const (
	LoungeStatusPending   = "pending"
	LoungeStatusActive    = "active"
	LoungeStatusInactive  = "inactive"
	LoungeStatusSuspended = "suspended"
)
