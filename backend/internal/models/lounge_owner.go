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
	RegistrationStep LoungeOwnerRegistrationStep `db:"registration_step" json:"registration_step"` // phone_verified, profile_submitted, lounge_added, completed
	ProfileCompleted bool                        `db:"profile_completed" json:"profile_completed"` // True when registration_step = 'completed' (but still pending admin approval)

	// Verification
	VerificationStatus LoungeOwnerVerificationStatus `db:"verification_status" json:"verification_status"` // pending, approved, rejected, suspended
	VerificationNotes  sql.NullString                `db:"verification_notes" json:"verification_notes,omitempty"`
	VerifiedAt         sql.NullTime                  `db:"verified_at" json:"verified_at,omitempty"`
	VerifiedBy         uuid.NullUUID                 `db:"verified_by" json:"verified_by,omitempty"`

	// Timestamps
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// LoungeOwnerVerificationStatus represents the verification status ENUM
type LoungeOwnerVerificationStatus string

const (
	LoungeOwnerVerificationPending   LoungeOwnerVerificationStatus = "pending"
	LoungeOwnerVerificationApproved  LoungeOwnerVerificationStatus = "approved"
	LoungeOwnerVerificationRejected  LoungeOwnerVerificationStatus = "rejected"
	LoungeOwnerVerificationSuspended LoungeOwnerVerificationStatus = "suspended"
)

// LoungeOwnerRegistrationStep represents the registration step ENUM
type LoungeOwnerRegistrationStep string

const (
	LoungeOwnerRegStepPhoneVerified    LoungeOwnerRegistrationStep = "phone_verified"
	LoungeOwnerRegStepProfileSubmitted LoungeOwnerRegistrationStep = "profile_submitted"
	LoungeOwnerRegStepLoungeAdded      LoungeOwnerRegistrationStep = "lounge_added"
	LoungeOwnerRegStepCompleted        LoungeOwnerRegistrationStep = "completed"
)

// Legacy constants for backward compatibility
const (
	RegStepPhoneVerified       = "phone_verified"
	RegStepBusinessInfo        = "profile_submitted" // Updated mapping
	RegStepLoungeAdded         = "lounge_added"
	RegStepCompleted           = "completed"
	LoungeVerificationPending  = "pending"
	LoungeVerificationApproved = "approved"
	LoungeVerificationRejected = "rejected"
)
