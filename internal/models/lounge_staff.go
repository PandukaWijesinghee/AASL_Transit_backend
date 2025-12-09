package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// LoungeStaff represents a staff member assigned to a lounge
type LoungeStaff struct {
	ID       uuid.UUID `db:"id" json:"id"`
	LoungeID uuid.UUID `db:"lounge_id" json:"lounge_id"`
	UserID   uuid.UUID `db:"user_id" json:"user_id"` // FK to users table

	// Personal Info (staff fills during registration)
	FullName  sql.NullString `db:"full_name" json:"full_name,omitempty"`
	NICNumber sql.NullString `db:"nic_number" json:"nic_number,omitempty"`
	Email     sql.NullString `db:"email" json:"email,omitempty"`

	// Registration Status
	ProfileCompleted bool `db:"profile_completed" json:"profile_completed"`

	// Employment Info
	EmploymentStatus LoungeStaffEmploymentStatus `db:"employment_status" json:"employment_status"` // active, terminated, suspended
	HiredDate        sql.NullTime                `db:"hired_date" json:"hired_date,omitempty"`
	TerminatedDate   sql.NullTime                `db:"terminated_date" json:"terminated_date,omitempty"`
	Notes            sql.NullString              `db:"notes" json:"notes,omitempty"`

	// Timestamps
	CreatedAt time.Time `db:"created_at" json:"created_at"` // Invitation time
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"` // Last update / registration completion
}

// LoungeStaffEmploymentStatus represents the employment status ENUM
type LoungeStaffEmploymentStatus string

const (
	LoungeStaffEmploymentActive     LoungeStaffEmploymentStatus = "active"
	LoungeStaffEmploymentTerminated LoungeStaffEmploymentStatus = "terminated"
	LoungeStaffEmploymentSuspended  LoungeStaffEmploymentStatus = "suspended"
)
