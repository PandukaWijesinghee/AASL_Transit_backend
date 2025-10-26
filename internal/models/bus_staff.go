package models

import (
	"time"
)

// StaffType represents the type of bus staff
type StaffType string

const (
	StaffTypeDriver    StaffType = "driver"
	StaffTypeConductor StaffType = "conductor"
)

// EmploymentStatus represents the employment status
type EmploymentStatus string

const (
	EmploymentStatusPending    EmploymentStatus = "pending"
	EmploymentStatusActive     EmploymentStatus = "active"
	EmploymentStatusInactive   EmploymentStatus = "inactive"
	EmploymentStatusSuspended  EmploymentStatus = "suspended"
	EmploymentStatusTerminated EmploymentStatus = "terminated"
)

// BackgroundCheckStatus represents background check status
type BackgroundCheckStatus string

const (
	BackgroundCheckPending  BackgroundCheckStatus = "pending"
	BackgroundCheckApproved BackgroundCheckStatus = "approved"
	BackgroundCheckRejected BackgroundCheckStatus = "rejected"
	BackgroundCheckExpired  BackgroundCheckStatus = "expired"
)

// BusStaff represents a driver or conductor
type BusStaff struct {
	ID                         string                `json:"id" db:"id"`
	UserID                     string                `json:"user_id" db:"user_id"`
	BusOwnerID                 *string               `json:"bus_owner_id,omitempty" db:"bus_owner_id"`
	StaffType                  StaffType             `json:"staff_type" db:"staff_type"`
	LicenseNumber              *string               `json:"license_number,omitempty" db:"license_number"`
	LicenseExpiryDate          *time.Time            `json:"license_expiry_date,omitempty" db:"license_expiry_date"`
	LicenseDocumentURL         *string               `json:"license_document_url,omitempty" db:"license_document_url"`
	ExperienceYears            int                   `json:"experience_years" db:"experience_years"`
	EmergencyContact           *string               `json:"emergency_contact,omitempty" db:"emergency_contact"`
	EmergencyContactName       *string               `json:"emergency_contact_name,omitempty" db:"emergency_contact_name"`
	MedicalCertificateExpiry   *time.Time            `json:"medical_certificate_expiry,omitempty" db:"medical_certificate_expiry"`
	MedicalCertificateURL      *string               `json:"medical_certificate_url,omitempty" db:"medical_certificate_url"`
	BackgroundCheckStatus      BackgroundCheckStatus `json:"background_check_status" db:"background_check_status"`
	BackgroundCheckDocumentURL *string               `json:"background_check_document_url,omitempty" db:"background_check_document_url"`
	EmploymentStatus           EmploymentStatus      `json:"employment_status" db:"employment_status"`
	HireDate                   *time.Time            `json:"hire_date,omitempty" db:"hire_date"`
	TerminationDate            *time.Time            `json:"termination_date,omitempty" db:"termination_date"`
	SalaryAmount               *float64              `json:"salary_amount,omitempty" db:"salary_amount"`
	PerformanceRating          float64               `json:"performance_rating" db:"performance_rating"`
	TotalTripsCompleted        int                   `json:"total_trips_completed" db:"total_trips_completed"`
	ProfileCompleted           bool                  `json:"profile_completed" db:"profile_completed"`
	VerificationNotes          *string               `json:"verification_notes,omitempty" db:"verification_notes"`
	VerifiedAt                 *time.Time            `json:"verified_at,omitempty" db:"verified_at"`
	VerifiedBy                 *string               `json:"verified_by,omitempty" db:"verified_by"`
	CreatedAt                  time.Time             `json:"created_at" db:"created_at"`
	UpdatedAt                  time.Time             `json:"updated_at" db:"updated_at"`
}

// StaffRegistrationInput represents input for staff registration
type StaffRegistrationInput struct {
	UserID                string    `json:"user_id" binding:"required"`
	StaffType             StaffType `json:"staff_type" binding:"required"`
	LicenseNumber         *string   `json:"license_number"`
	LicenseExpiryDate     *string   `json:"license_expiry_date"`
	ExperienceYears       int       `json:"experience_years"`
	EmergencyContact      string    `json:"emergency_contact" binding:"required"`
	EmergencyContactName  string    `json:"emergency_contact_name" binding:"required"`
	BusOwnerCode          *string   `json:"bus_owner_code"`
	BusRegistrationNumber *string   `json:"bus_registration_number"`
}

// StaffProfileUpdate represents input for profile updates
type StaffProfileUpdate struct {
	LicenseNumber            *string `json:"license_number"`
	LicenseExpiryDate        *string `json:"license_expiry_date"`
	ExperienceYears          *int    `json:"experience_years"`
	EmergencyContact         *string `json:"emergency_contact"`
	EmergencyContactName     *string `json:"emergency_contact_name"`
	MedicalCertificateExpiry *string `json:"medical_certificate_expiry"`
}

// CompleteStaffProfile represents complete profile with user and bus owner info
type CompleteStaffProfile struct {
	User     *User     `json:"user"`
	Staff    *BusStaff `json:"staff"`
	BusOwner *BusOwner `json:"bus_owner,omitempty"`
}

// AddStaffRequest represents request from bus owner to add staff
type AddStaffRequest struct {
	PhoneNumber          string    `json:"phone_number" binding:"required"`
	FirstName            string    `json:"first_name" binding:"required"`
	LastName             string    `json:"last_name" binding:"required"`
	StaffType            StaffType `json:"staff_type" binding:"required"`
	NTCLicenseNumber     string    `json:"ntc_license_number" binding:"required"` // MANDATORY for both driver and conductor
	LicenseExpiryDate    string    `json:"license_expiry_date" binding:"required"` // MANDATORY
	ExperienceYears      int       `json:"experience_years"`
	EmergencyContact     string    `json:"emergency_contact"`
	EmergencyContactName string    `json:"emergency_contact_name"`
}
