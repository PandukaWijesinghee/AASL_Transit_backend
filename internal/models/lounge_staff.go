package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// LoungeStaff represents a staff member assigned to a lounge
type LoungeStaff struct {
	ID       uuid.UUID     `db:"id" json:"id"`
	LoungeID uuid.UUID     `db:"lounge_id" json:"lounge_id"`
	UserID   uuid.NullUUID `db:"user_id" json:"user_id,omitempty"` // NULL until they register

	// Staff Information
	PhoneNumber string         `db:"phone_number" json:"phone_number"`
	FullName    sql.NullString `db:"full_name" json:"full_name,omitempty"`
	NICNumber   sql.NullString `db:"nic_number" json:"nic_number,omitempty"`
	NICFrontURL sql.NullString `db:"nic_front_url" json:"nic_front_url,omitempty"`
	NICBackURL  sql.NullString `db:"nic_back_url" json:"nic_back_url,omitempty"`
	Email       sql.NullString `db:"email" json:"email,omitempty"`

	// Permissions
	PermissionType string `db:"permission_type" json:"permission_type"` // 'admin' or 'staff'

	// Employment
	EmploymentStatus string       `db:"employment_status" json:"employment_status"` // pending, active, inactive, suspended
	HiredDate        sql.NullTime `db:"hired_date" json:"hired_date,omitempty"`
	TerminatedDate   sql.NullTime `db:"terminated_date" json:"terminated_date,omitempty"`

	// Registration
	HasRegistered bool         `db:"has_registered" json:"has_registered"`
	InvitedAt     time.Time    `db:"invited_at" json:"invited_at"`
	RegisteredAt  sql.NullTime `db:"registered_at" json:"registered_at,omitempty"`

	// Metadata
	Notes     sql.NullString `db:"notes" json:"notes,omitempty"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at" json:"updated_at"`
}

// Permission type constants for lounge staff
const (
	LoungePermissionTypeAdmin = "admin" // Full access to lounge management
	LoungePermissionTypeStaff = "staff" // View-only access to operations
)

// Lounge staff employment status constants
const (
	StaffStatusActive    = "active"
	StaffStatusInactive  = "inactive"
	StaffStatusSuspended = "suspended"
)
