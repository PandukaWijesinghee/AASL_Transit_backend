package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Lounge represents a lounge location registered by a lounge owner
type Lounge struct {
	ID              uuid.UUID      `db:"id" json:"id"`
	LoungeOwnerID   uuid.UUID      `db:"lounge_owner_id" json:"lounge_owner_id"`
	LoungeName      string         `db:"lounge_name" json:"lounge_name"`
	BusinessLicense sql.NullString `db:"business_license_number" json:"business_license_number,omitempty"`

	// Location
	FullAddress string         `db:"full_address" json:"full_address"`
	City        string         `db:"city" json:"city"`
	State       sql.NullString `db:"state" json:"state,omitempty"`
	PostalCode  sql.NullString `db:"postal_code" json:"postal_code,omitempty"`
	Latitude    sql.NullString `db:"latitude" json:"latitude,omitempty"`
	Longitude   sql.NullString `db:"longitude" json:"longitude,omitempty"`

	// Contact
	ContactPersonName sql.NullString `db:"contact_person_name" json:"contact_person_name,omitempty"`
	BusinessEmail     sql.NullString `db:"business_email" json:"business_email,omitempty"`
	BusinessPhone     sql.NullString `db:"business_phone" json:"business_phone,omitempty"`

	// Media & Facilities (stored as JSONB)
	LoungePhotos   []byte         `db:"lounge_photos" json:"lounge_photos,omitempty"`     // JSONB
	Facilities     []byte         `db:"facilities" json:"facilities,omitempty"`           // JSONB
	OperatingHours []byte         `db:"operating_hours" json:"operating_hours,omitempty"` // JSONB
	Description    sql.NullString `db:"description" json:"description,omitempty"`

	// Verification
	VerificationStatus string         `db:"verification_status" json:"verification_status"` // pending, approved, rejected, suspended
	VerificationNotes  sql.NullString `db:"verification_notes" json:"verification_notes,omitempty"`
	VerifiedAt         sql.NullTime   `db:"verified_at" json:"verified_at,omitempty"`
	VerifiedBy         uuid.NullUUID  `db:"verified_by" json:"verified_by,omitempty"`

	// Metadata
	IsActive  bool      `db:"is_active" json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// LoungePhoto represents a single photo in the lounge photos array
type LoungePhoto struct {
	URL   string `json:"url"`
	Order int    `json:"order"`
}

// OperatingHours represents operating hours for a specific day
type OperatingHours struct {
	Monday    *DayHours `json:"monday,omitempty"`
	Tuesday   *DayHours `json:"tuesday,omitempty"`
	Wednesday *DayHours `json:"wednesday,omitempty"`
	Thursday  *DayHours `json:"thursday,omitempty"`
	Friday    *DayHours `json:"friday,omitempty"`
	Saturday  *DayHours `json:"saturday,omitempty"`
	Sunday    *DayHours `json:"sunday,omitempty"`
}

// DayHours represents opening and closing time for a day
type DayHours struct {
	Open  string `json:"open"`  // Format: "HH:MM"
	Close string `json:"close"` // Format: "HH:MM"
}

// VerificationStatus constants
const (
	LoungeVerificationPending   = "pending"
	LoungeVerificationApproved  = "approved"
	LoungeVerificationRejected  = "rejected"
	LoungeVerificationSuspended = "suspended"
)
