package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// LoungeOwner represents a lounge owner in the system
type LoungeOwner struct {
	ID     uuid.UUID `db:"id" json:"id"`
	UserID uuid.UUID `db:"user_id" json:"user_id"`

	// Business Information
	BusinessName    sql.NullString `db:"business_name" json:"business_name,omitempty"`       // Business/Hotel name
	BusinessLicense sql.NullString `db:"business_license" json:"business_license,omitempty"` // Business registration number

	// Manager Information (person managing the lounges)
	ManagerFullName  sql.NullString `db:"manager_full_name" json:"manager_full_name,omitempty"`   // Manager's full legal name
	ManagerNICNumber sql.NullString `db:"manager_nic_number" json:"manager_nic_number,omitempty"` // Manager's NIC (UNIQUE per person)
	ManagerEmail     sql.NullString `db:"manager_email" json:"manager_email,omitempty"`           // Manager's email (optional)

	// Registration Progress Tracking
	RegistrationStep string `db:"registration_step" json:"registration_step"` // phone_verified, business_info, lounge_added, completed (no NIC verification step)
	ProfileCompleted bool   `db:"profile_completed" json:"profile_completed"` // True when registration_step = 'completed' (but still pending admin approval)

	// Verification
	VerificationStatus string         `db:"verification_status" json:"verification_status"` // pending, approved, rejected
	VerificationNotes  sql.NullString `db:"verification_notes" json:"verification_notes,omitempty"`
	VerifiedAt         sql.NullTime   `db:"verified_at" json:"verified_at,omitempty"`
	VerifiedBy         uuid.NullUUID  `db:"verified_by" json:"verified_by,omitempty"`

	// Timestamps
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Registration step constants
// New flow: phone_verified -> business_info -> lounge_added -> completed
// Note: nic_uploaded step has been removed (NIC images collected with business_info, verified by admin)
const (
	RegStepPhoneVerified = "phone_verified"
	RegStepBusinessInfo  = "business_info"
	RegStepLoungeAdded   = "lounge_added"
	RegStepCompleted     = "completed"
)

// Lounge owner verification status constants (for admin approval)
const (
	LoungeVerificationPending  = "pending"
	LoungeVerificationApproved = "approved"
	LoungeVerificationRejected = "rejected"
)
