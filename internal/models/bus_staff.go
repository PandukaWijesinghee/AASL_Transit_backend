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
	EmploymentStatusTerminated EmploymentStatus = "terminated"
	EmploymentStatusResigned   EmploymentStatus = "resigned"
	EmploymentStatusSuspended  EmploymentStatus = "suspended"
)

// StaffVerificationStatus represents staff verification status
type StaffVerificationStatus string

const (
	StaffVerificationPending  StaffVerificationStatus = "pending"
	StaffVerificationApproved StaffVerificationStatus = "approved"
	StaffVerificationRejected StaffVerificationStatus = "rejected"
)

// BusStaff represents a driver or conductor (profile only, no employment details)
type BusStaff struct {
	ID                       string                `json:"id" db:"id"`
	UserID                   string                `json:"user_id" db:"user_id"`
	FirstName                *string               `json:"first_name,omitempty" db:"first_name"`
	LastName                 *string               `json:"last_name,omitempty" db:"last_name"`
	StaffType                StaffType             `json:"staff_type" db:"staff_type"`
	LicenseNumber            *string               `json:"license_number,omitempty" db:"license_number"`
	LicenseExpiryDate        *time.Time            `json:"license_expiry_date,omitempty" db:"license_expiry_date"`
	ExperienceYears          int                   `json:"experience_years" db:"experience_years"`
	EmergencyContact         *string                 `json:"emergency_contact,omitempty" db:"emergency_contact"`
	EmergencyContactName     *string                 `json:"emergency_contact_name,omitempty" db:"emergency_contact_name"`
	ProfileCompleted         bool                    `json:"profile_completed" db:"profile_completed"`
	IsVerified               bool                    `json:"is_verified" db:"is_verified"`
	VerificationStatus       StaffVerificationStatus `json:"verification_status" db:"verification_status"`
	VerificationNotes        *string                 `json:"verification_notes,omitempty" db:"verification_notes"`
	VerifiedAt               *time.Time              `json:"verified_at,omitempty" db:"verified_at"`
	VerifiedBy               *string                 `json:"verified_by,omitempty" db:"verified_by"`
	CreatedAt                time.Time             `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time             `json:"updated_at" db:"updated_at"`
}

// BusStaffEmployment represents employment history of a staff member with a bus owner
type BusStaffEmployment struct {
	ID                  string           `json:"id" db:"id"`
	StaffID             string           `json:"staff_id" db:"staff_id"`
	BusOwnerID          string           `json:"bus_owner_id" db:"bus_owner_id"`
	EmploymentStatus    EmploymentStatus `json:"employment_status" db:"employment_status"`
	HireDate            *time.Time       `json:"hire_date,omitempty" db:"hire_date"`
	TerminationDate     *time.Time       `json:"termination_date,omitempty" db:"termination_date"`
	TerminationReason   *string          `json:"termination_reason,omitempty" db:"termination_reason"`
	SalaryAmount        *float64         `json:"salary_amount,omitempty" db:"salary_amount"`
	PerformanceRating   float64          `json:"performance_rating" db:"performance_rating"`
	TotalTripsCompleted int              `json:"total_trips_completed" db:"total_trips_completed"`
	IsCurrent           bool             `json:"is_current" db:"is_current"`
	Notes               *string          `json:"notes,omitempty" db:"notes"`
	CreatedAt           time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at" db:"updated_at"`
}

// StaffWithEmployment combines staff profile with current employment details
type StaffWithEmployment struct {
	Staff      *BusStaff           `json:"staff"`
	Employment *BusStaffEmployment `json:"employment,omitempty"`
}

// StaffRegistrationInput represents input for staff registration
type StaffRegistrationInput struct {
	UserID               string    `json:"user_id" binding:"required"`
	FirstName            string    `json:"first_name" binding:"required"`
	LastName             string    `json:"last_name" binding:"required"`
	StaffType            StaffType `json:"staff_type" binding:"required"`
	LicenseNumber        *string   `json:"license_number"`
	LicenseExpiryDate    *string   `json:"license_expiry_date"`
	ExperienceYears      int       `json:"experience_years"`
	EmergencyContact     string    `json:"emergency_contact" binding:"required"`
	EmergencyContactName string    `json:"emergency_contact_name" binding:"required"`
}

// StaffProfileUpdate represents input for profile updates
type StaffProfileUpdate struct {
	FirstName            *string `json:"first_name"`
	LastName             *string `json:"last_name"`
	LicenseNumber        *string `json:"license_number"`
	LicenseExpiryDate    *string `json:"license_expiry_date"`
	ExperienceYears      *int    `json:"experience_years"`
	EmergencyContact     *string `json:"emergency_contact"`
	EmergencyContactName *string `json:"emergency_contact_name"`
}

// CompleteStaffProfile represents complete profile with user, staff, and employment info
type CompleteStaffProfile struct {
	User       *User               `json:"user"`
	Staff      *BusStaff           `json:"staff"`
	Employment *BusStaffEmployment `json:"employment,omitempty"`
	BusOwner   *BusOwner           `json:"bus_owner,omitempty"`
}

// AddStaffRequest represents request from bus owner to add staff (DEPRECATED - use verify/link flow)
type AddStaffRequest struct {
	PhoneNumber          string    `json:"phone_number" binding:"required"`
	FirstName            string    `json:"first_name" binding:"required"`
	LastName             string    `json:"last_name" binding:"required"`
	StaffType            StaffType `json:"staff_type" binding:"required"`
	NTCLicenseNumber     string    `json:"ntc_license_number" binding:"required"`
	LicenseExpiryDate    string    `json:"license_expiry_date" binding:"required"`
	ExperienceYears      int       `json:"experience_years"`
	EmergencyContact     string    `json:"emergency_contact"`
	EmergencyContactName string    `json:"emergency_contact_name"`
}

// VerifyStaffRequest represents request to verify if a staff can be added
type VerifyStaffRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

// VerifyStaffResponse represents response for staff verification
type VerifyStaffResponse struct {
	Found            bool       `json:"found"`
	Eligible         bool       `json:"eligible"`
	StaffID          string     `json:"staff_id,omitempty"`
	StaffType        *StaffType `json:"staff_type,omitempty"`
	FirstName        string     `json:"first_name,omitempty"`
	LastName         string     `json:"last_name,omitempty"`
	ProfileCompleted bool       `json:"profile_completed"`
	IsVerified       bool       `json:"is_verified"`
	AlreadyLinked    bool       `json:"already_linked"`
	CurrentOwnerID   *string    `json:"current_owner_id,omitempty"`
	Message          string     `json:"message"`
	Reason           string     `json:"reason,omitempty"`
}

// LinkStaffRequest represents request to link verified staff to bus owner
type LinkStaffRequest struct {
	StaffID string `json:"staff_id" binding:"required"`
}

// UnlinkStaffRequest represents request to end staff employment
type UnlinkStaffRequest struct {
	StaffID           string `json:"staff_id" binding:"required"`
	TerminationReason string `json:"termination_reason"`
	Status            string `json:"status"` // "terminated" or "resigned"
}
