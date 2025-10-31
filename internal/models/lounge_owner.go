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

	// Personal Information
	FullName sql.NullString `db:"full_name" json:"full_name,omitempty"` // Name as on NIC

	// NIC Information
	NICNumber        sql.NullString `db:"nic_number" json:"nic_number,omitempty"`
	NICFrontImageURL sql.NullString `db:"nic_front_image_url" json:"nic_front_image_url,omitempty"`
	NICBackImageURL  sql.NullString `db:"nic_back_image_url" json:"nic_back_image_url,omitempty"`
	NICOCRAttempts   int            `db:"nic_ocr_attempts" json:"nic_ocr_attempts"`
	LastOCRAttemptAt sql.NullTime   `db:"last_ocr_attempt_at" json:"last_ocr_attempt_at,omitempty"`
	OCRBlockedUntil  sql.NullTime   `db:"ocr_blocked_until" json:"ocr_blocked_until,omitempty"`

	// Registration Progress Tracking
	RegistrationStep string `db:"registration_step" json:"registration_step"` // phone_verified, personal_info, nic_uploaded, lounge_added, completed

	// Business Information (Optional during registration, required later)
	CompanyName            sql.NullString `db:"company_name" json:"company_name,omitempty"`
	LicenseNumber          sql.NullString `db:"license_number" json:"license_number,omitempty"`
	ContactPerson          sql.NullString `db:"contact_person" json:"contact_person,omitempty"`
	Address                sql.NullString `db:"address" json:"address,omitempty"`
	City                   sql.NullString `db:"city" json:"city,omitempty"`
	State                  sql.NullString `db:"state" json:"state,omitempty"`
	Country                sql.NullString `db:"country" json:"country,omitempty"`
	PostalCode             sql.NullString `db:"postal_code" json:"postal_code,omitempty"`
	BusinessEmail          sql.NullString `db:"business_email" json:"business_email,omitempty"`
	BusinessPhone          sql.NullString `db:"business_phone" json:"business_phone,omitempty"`
	TaxID                  sql.NullString `db:"tax_id" json:"tax_id,omitempty"`
	BankAccountDetails     []byte         `db:"bank_account_details" json:"bank_account_details,omitempty"`     // JSONB
	VerificationDocuments  []byte         `db:"verification_documents" json:"verification_documents,omitempty"` // JSONB
	VerificationStatus     string         `db:"verification_status" json:"verification_status"`                 // pending, approved, rejected
	TotalLounges           int            `db:"total_lounges" json:"total_lounges"`
	ProfileCompleted       bool           `db:"profile_completed" json:"profile_completed"`
	IdentityOrIncorpNumber sql.NullString `db:"identity_or_incorporation_no" json:"identity_or_incorporation_no,omitempty"`

	// Metadata
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Registration step constants
const (
	RegStepPhoneVerified = "phone_verified"
	RegStepPersonalInfo  = "personal_info"
	RegStepNICUploaded   = "nic_uploaded"
	RegStepLoungeAdded   = "lounge_added"
	RegStepCompleted     = "completed"
)

// Max OCR attempts before blocking
const MaxOCRAttempts = 4

// OCR block duration (24 hours)
const OCRBlockDuration = 24 * time.Hour
