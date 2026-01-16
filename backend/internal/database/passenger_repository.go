package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// PassengerRepository handles passenger database operations
type PassengerRepository struct {
	db DB
}

// NewPassengerRepository creates a new passenger repository
func NewPassengerRepository(db DB) *PassengerRepository {
	return &PassengerRepository{
		db: db,
	}
}

// CreatePassenger creates a new passenger record
func (r *PassengerRepository) CreatePassenger(userID uuid.UUID) (*models.Passenger, error) {
	passenger := &models.Passenger{
		ID:                 uuid.New(),
		UserID:             userID,
		ProfileCompleted:   false,
		TotalTrips:         0,
		LoyaltyPoints:      0,
		VerificationStatus: "pending",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	query := `
		INSERT INTO passengers (
			id, user_id, profile_completed, total_trips, 
			loyalty_points, verification_status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(
		query,
		passenger.ID,
		passenger.UserID,
		passenger.ProfileCompleted,
		passenger.TotalTrips,
		passenger.LoyaltyPoints,
		passenger.VerificationStatus,
		passenger.CreatedAt,
		passenger.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create passenger: %w", err)
	}

	return passenger, nil
}

// GetPassengerByUserID retrieves a passenger by user ID
func (r *PassengerRepository) GetPassengerByUserID(userID uuid.UUID) (*models.Passenger, error) {
	var passenger models.Passenger

	query := `
		SELECT id, user_id, first_name, last_name, email, date_of_birth,
		       nic, address, city, postal_code, profile_photo_url,
		       profile_completed, emergency_contact_name, emergency_contact_phone,
		       preferred_seat_type, special_requirements, total_trips,
		       loyalty_points, verification_status, verification_notes,
		       verified_at, verified_by, created_at, updated_at
		FROM passengers
		WHERE user_id = $1
	`

	err := r.db.Get(&passenger, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get passenger by user ID: %w", err)
	}

	return &passenger, nil
}

// GetPassengerByID retrieves a passenger by ID
func (r *PassengerRepository) GetPassengerByID(id uuid.UUID) (*models.Passenger, error) {
	var passenger models.Passenger

	query := `
		SELECT id, user_id, first_name, last_name, email, date_of_birth,
		       nic, address, city, postal_code, profile_photo_url,
		       profile_completed, emergency_contact_name, emergency_contact_phone,
		       preferred_seat_type, special_requirements, total_trips,
		       loyalty_points, verification_status, verification_notes,
		       verified_at, verified_by, created_at, updated_at
		FROM passengers
		WHERE id = $1
	`

	err := r.db.Get(&passenger, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get passenger by ID: %w", err)
	}

	return &passenger, nil
}

// GetOrCreatePassenger gets existing passenger or creates a new one
func (r *PassengerRepository) GetOrCreatePassenger(userID uuid.UUID) (*models.Passenger, bool, error) {
	// Try to get existing passenger
	passenger, err := r.GetPassengerByUserID(userID)
	if err != nil {
		return nil, false, err
	}

	// If passenger exists, return it
	if passenger != nil {
		return passenger, false, nil
	}

	// Create new passenger
	passenger, err = r.CreatePassenger(userID)
	if err != nil {
		return nil, false, err
	}

	return passenger, true, nil
}

// UpdatePassengerNames updates first_name and last_name
func (r *PassengerRepository) UpdatePassengerNames(userID uuid.UUID, firstName, lastName string) error {
	query := `
		UPDATE passengers
		SET first_name = $1,
		    last_name = $2,
		    updated_at = $3
		WHERE user_id = $4
	`

	result, err := r.db.Exec(query, firstName, lastName, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update passenger names: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("passenger not found for user")
	}

	return nil
}

// SetPassengerProfileCompleted sets profile_completed status
func (r *PassengerRepository) SetPassengerProfileCompleted(userID uuid.UUID, completed bool) error {
	query := `
		UPDATE passengers
		SET profile_completed = $1,
		    updated_at = $2
		WHERE user_id = $3
	`

	result, err := r.db.Exec(query, completed, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to set passenger profile completion: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("passenger not found for user")
	}

	return nil
}

// UpdatePassengerProfile updates full passenger profile
func (r *PassengerRepository) UpdatePassengerProfile(userID uuid.UUID, firstName, lastName, email, address, city, postalCode string) error {
	query := `
		UPDATE passengers
		SET first_name = $1,
		    last_name = $2,
		    email = $3,
		    address = $4,
		    city = $5,
		    postal_code = $6,
		    updated_at = $7
		WHERE user_id = $8
	`

	result, err := r.db.Exec(query, firstName, lastName, email, address, city, postalCode, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update passenger profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("passenger not found for user")
	}

	return nil
}

// UpdatePassengerEmergencyContact updates emergency contact information
func (r *PassengerRepository) UpdatePassengerEmergencyContact(userID uuid.UUID, contactName, contactPhone string) error {
	query := `
		UPDATE passengers
		SET emergency_contact_name = $1,
		    emergency_contact_phone = $2,
		    updated_at = $3
		WHERE user_id = $4
	`

	_, err := r.db.Exec(query, contactName, contactPhone, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update emergency contact: %w", err)
	}

	return nil
}

// UpdatePassengerPreferences updates passenger preferences
func (r *PassengerRepository) UpdatePassengerPreferences(userID uuid.UUID, preferredSeatType, specialRequirements string) error {
	query := `
		UPDATE passengers
		SET preferred_seat_type = $1,
		    special_requirements = $2,
		    updated_at = $3
		WHERE user_id = $4
	`

	_, err := r.db.Exec(query, preferredSeatType, specialRequirements, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update passenger preferences: %w", err)
	}

	return nil
}

// IncrementTotalTrips increases the total trips count
func (r *PassengerRepository) IncrementTotalTrips(userID uuid.UUID) error {
	query := `
		UPDATE passengers
		SET total_trips = total_trips + 1,
		    updated_at = $1
		WHERE user_id = $2
	`

	_, err := r.db.Exec(query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to increment total trips: %w", err)
	}

	return nil
}

// AddLoyaltyPoints adds loyalty points to passenger
func (r *PassengerRepository) AddLoyaltyPoints(userID uuid.UUID, points int) error {
	query := `
		UPDATE passengers
		SET loyalty_points = loyalty_points + $1,
		    updated_at = $2
		WHERE user_id = $3
	`

	_, err := r.db.Exec(query, points, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to add loyalty points: %w", err)
	}

	return nil
}

// GetPassengerWithUser retrieves passenger with user data
func (r *PassengerRepository) GetPassengerWithUser(userID uuid.UUID) (*models.PassengerWithUser, error) {
	var result models.PassengerWithUser

	query := `
		SELECT p.id, p.user_id, p.first_name, p.last_name, p.email, p.date_of_birth,
		       p.nic, p.address, p.city, p.postal_code, p.profile_photo_url,
		       p.profile_completed, p.emergency_contact_name, p.emergency_contact_phone,
		       p.preferred_seat_type, p.special_requirements, p.total_trips,
		       p.loyalty_points, p.verification_status, p.verification_notes,
		       p.verified_at, p.verified_by, p.created_at, p.updated_at,
		       u.phone, u.phone_verified, u.status as user_status
		FROM passengers p
		JOIN users u ON p.user_id = u.id
		WHERE p.user_id = $1
	`

	err := r.db.Get(&result, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get passenger with user: %w", err)
	}

	return &result, nil
}

// ListPassengers retrieves all passengers with pagination
func (r *PassengerRepository) ListPassengers(limit, offset int) ([]*models.PassengerWithUser, error) {
	var passengers []*models.PassengerWithUser

	query := `
		SELECT p.id, p.user_id, p.first_name, p.last_name, p.email, p.date_of_birth,
		       p.nic, p.address, p.city, p.postal_code, p.profile_photo_url,
		       p.profile_completed, p.emergency_contact_name, p.emergency_contact_phone,
		       p.preferred_seat_type, p.special_requirements, p.total_trips,
		       p.loyalty_points, p.verification_status, p.verification_notes,
		       p.verified_at, p.verified_by, p.created_at, p.updated_at,
		       u.phone, u.phone_verified, u.status as user_status
		FROM passengers p
		JOIN users u ON p.user_id = u.id
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`

	err := r.db.Select(&passengers, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list passengers: %w", err)
	}

	return passengers, nil
}

// CountPassengers returns the total number of passengers
func (r *PassengerRepository) CountPassengers() (int, error) {
	var count int

	query := `SELECT COUNT(*) FROM passengers`

	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count passengers: %w", err)
	}

	return count, nil
}

// IsPassengerProfileComplete checks if passenger profile has required fields
func (r *PassengerRepository) IsPassengerProfileComplete(userID uuid.UUID) (bool, error) {
	var profileCompleted bool

	query := `
		SELECT profile_completed
		FROM passengers
		WHERE user_id = $1
	`

	err := r.db.QueryRow(query, userID).Scan(&profileCompleted)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // No passenger record means incomplete
		}
		return false, fmt.Errorf("failed to check passenger profile completion: %w", err)
	}

	return profileCompleted, nil
}
