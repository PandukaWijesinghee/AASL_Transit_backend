package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// VerificationStatus represents verification status
type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "pending"
	VerificationVerified VerificationStatus = "verified"
	VerificationRejected VerificationStatus = "rejected"
)

// JSONB is a custom type for handling JSONB fields
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
// Returns JSON as string for compatibility with pgx simple protocol mode
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// BusOwner represents a bus company owner
type BusOwner struct {
	ID                         string             `json:"id" db:"id"`
	UserID                     string             `json:"user_id" db:"user_id"`
	CompanyName                *string            `json:"company_name,omitempty" db:"company_name"`
	LicenseNumber              *string            `json:"license_number,omitempty" db:"license_number"` // DEPRECATED: Use IdentityOrIncorporationNo
	IdentityOrIncorporationNo  *string            `json:"identity_or_incorporation_no,omitempty" db:"identity_or_incorporation_no"`
	ContactPerson              *string            `json:"contact_person,omitempty" db:"contact_person"`
	Address                    *string            `json:"address,omitempty" db:"address"`
	City                       *string            `json:"city,omitempty" db:"city"`
	State                      *string            `json:"state,omitempty" db:"state"`
	Country                    string             `json:"country" db:"country"`
	PostalCode                 *string            `json:"postal_code,omitempty" db:"postal_code"`
	VerificationStatus         VerificationStatus `json:"verification_status" db:"verification_status"`
	VerificationDocuments      JSONB              `json:"verification_documents,omitempty" db:"verification_documents"`
	BusinessEmail              *string            `json:"business_email,omitempty" db:"business_email"`
	BusinessPhone              *string            `json:"business_phone,omitempty" db:"business_phone"`
	TaxID                      *string            `json:"tax_id,omitempty" db:"tax_id"`
	BankAccountDetails         JSONB              `json:"bank_account_details,omitempty" db:"bank_account_details"`
	TotalBuses                 int                `json:"total_buses" db:"total_buses"`
	ProfileCompleted           bool               `json:"profile_completed" db:"profile_completed"`
	CreatedAt                  time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt                  time.Time          `json:"updated_at" db:"updated_at"`
}

// BusOwnerPublicInfo represents public information about a bus owner (for search results)
type BusOwnerPublicInfo struct {
	ID                 string             `json:"id"`
	CompanyName        *string            `json:"company_name,omitempty"`
	ContactPerson      *string            `json:"contact_person,omitempty"`
	City               *string            `json:"city,omitempty"`
	VerificationStatus VerificationStatus `json:"verification_status"`
	TotalBuses         int                `json:"total_buses"`
}
