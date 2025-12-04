package models

import (
	"time"

	"github.com/google/uuid"
)

// Passenger represents a passenger profile in the system
// This is separate from User (which handles authentication)
// Same user can be both a passenger and a driver/bus_owner/etc.
type Passenger struct {
	ID                    uuid.UUID  `json:"id" db:"id"`
	UserID                uuid.UUID  `json:"user_id" db:"user_id"`
	FirstName             NullString `json:"first_name,omitempty" db:"first_name"`
	LastName              NullString `json:"last_name,omitempty" db:"last_name"`
	Email                 NullString `json:"email,omitempty" db:"email"`
	DateOfBirth           NullTime   `json:"date_of_birth,omitempty" db:"date_of_birth"`
	NIC                   NullString `json:"nic,omitempty" db:"nic"`
	Address               NullString `json:"address,omitempty" db:"address"`
	City                  NullString `json:"city,omitempty" db:"city"`
	PostalCode            NullString `json:"postal_code,omitempty" db:"postal_code"`
	ProfilePhotoURL       NullString `json:"profile_photo_url,omitempty" db:"profile_photo_url"`
	ProfileCompleted      bool       `json:"profile_completed" db:"profile_completed"`
	EmergencyContactName  NullString `json:"emergency_contact_name,omitempty" db:"emergency_contact_name"`
	EmergencyContactPhone NullString `json:"emergency_contact_phone,omitempty" db:"emergency_contact_phone"`
	PreferredSeatType     NullString `json:"preferred_seat_type,omitempty" db:"preferred_seat_type"`
	SpecialRequirements   NullString `json:"special_requirements,omitempty" db:"special_requirements"`
	TotalTrips            int        `json:"total_trips" db:"total_trips"`
	LoyaltyPoints         int        `json:"loyalty_points" db:"loyalty_points"`
	VerificationStatus    string     `json:"verification_status" db:"verification_status"`
	VerificationNotes     NullString `json:"verification_notes,omitempty" db:"verification_notes"`
	VerifiedAt            NullTime   `json:"verified_at,omitempty" db:"verified_at"`
	VerifiedBy            *uuid.UUID `json:"verified_by,omitempty" db:"verified_by"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
}

// PassengerWithUser combines Passenger with User data for API responses
type PassengerWithUser struct {
	Passenger
	Phone         string `json:"phone"`
	PhoneVerified bool   `json:"phone_verified"`
	Status        string `json:"user_status"`
}
