package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeRepository handles database operations for lounges
type LoungeRepository struct {
	db *sqlx.DB
}

// NewLoungeRepository creates a new lounge repository
func NewLoungeRepository(db *sqlx.DB) *LoungeRepository {
	return &LoungeRepository{db: db}
}

// CreateLoungeRequest represents the data needed to create a lounge
type CreateLoungeRequest struct {
	LoungeOwnerID     uuid.UUID
	LoungeName        string
	BusinessLicense   *string
	FullAddress       string
	City              string
	State             *string
	PostalCode        *string
	Latitude          *string
	Longitude         *string
	ContactPersonName *string
	BusinessEmail     *string
	BusinessPhone     *string
	LoungePhotos      []models.LoungePhoto // Array of photo objects
	Facilities        []string             // Array of facility names
	OperatingHours    *models.OperatingHours
	Description       *string
}

// CreateLounge creates a new lounge
func (r *LoungeRepository) CreateLounge(req CreateLoungeRequest) (*models.Lounge, error) {
	lounge := &models.Lounge{
		ID:                 uuid.New(),
		LoungeOwnerID:      req.LoungeOwnerID,
		LoungeName:         req.LoungeName,
		FullAddress:        req.FullAddress,
		City:               req.City,
		VerificationStatus: models.LoungeVerificationPending,
		IsActive:           true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Convert photos array to JSONB
	photosJSON, err := json.Marshal(req.LoungePhotos)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lounge photos: %w", err)
	}

	// Convert facilities array to JSONB
	facilitiesJSON, err := json.Marshal(req.Facilities)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal facilities: %w", err)
	}

	// Convert operating hours to JSONB if provided
	var operatingHoursJSON []byte
	if req.OperatingHours != nil {
		operatingHoursJSON, err = json.Marshal(req.OperatingHours)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal operating hours: %w", err)
		}
	}

	query := `
		INSERT INTO lounges (
			id, lounge_owner_id, lounge_name, business_license_number,
			full_address, city, state, postal_code, latitude, longitude,
			contact_person_name, business_email, business_phone,
			lounge_photos, facilities, operating_hours, description,
			verification_status, is_active, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)
		RETURNING id, created_at, updated_at
	`

	err = r.db.QueryRowx(
		query,
		lounge.ID,
		lounge.LoungeOwnerID,
		req.LoungeName,
		req.BusinessLicense,
		req.FullAddress,
		req.City,
		req.State,
		req.PostalCode,
		req.Latitude,
		req.Longitude,
		req.ContactPersonName,
		req.BusinessEmail,
		req.BusinessPhone,
		photosJSON,
		facilitiesJSON,
		operatingHoursJSON,
		req.Description,
		lounge.VerificationStatus,
		lounge.IsActive,
		lounge.CreatedAt,
		lounge.UpdatedAt,
	).Scan(&lounge.ID, &lounge.CreatedAt, &lounge.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create lounge: %w", err)
	}

	return lounge, nil
}

// GetLoungeByID retrieves a lounge by ID
func (r *LoungeRepository) GetLoungeByID(id uuid.UUID) (*models.Lounge, error) {
	var lounge models.Lounge

	query := `SELECT * FROM lounges WHERE id = $1`

	err := r.db.Get(&lounge, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lounge: %w", err)
	}

	return &lounge, nil
}

// GetLoungesByOwnerID retrieves all lounges for a lounge owner
func (r *LoungeRepository) GetLoungesByOwnerID(ownerID uuid.UUID) ([]*models.Lounge, error) {
	var lounges []*models.Lounge

	query := `
		SELECT * FROM lounges 
		WHERE lounge_owner_id = $1 
		ORDER BY created_at DESC
	`

	err := r.db.Select(&lounges, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges: %w", err)
	}

	return lounges, nil
}

// GetApprovedLounges retrieves all approved and active lounges
func (r *LoungeRepository) GetApprovedLounges() ([]*models.Lounge, error) {
	var lounges []*models.Lounge

	query := `
		SELECT * FROM lounges 
		WHERE verification_status = $1 AND is_active = true
		ORDER BY lounge_name ASC
	`

	err := r.db.Select(&lounges, query, models.LoungeVerificationApproved)
	if err != nil {
		return nil, fmt.Errorf("failed to get approved lounges: %w", err)
	}

	return lounges, nil
}

// GetLoungesByCity retrieves approved lounges in a specific city
func (r *LoungeRepository) GetLoungesByCity(city string) ([]*models.Lounge, error) {
	var lounges []*models.Lounge

	query := `
		SELECT * FROM lounges 
		WHERE city ILIKE $1 
			AND verification_status = $2 
			AND is_active = true
		ORDER BY lounge_name ASC
	`

	err := r.db.Select(&lounges, query, "%"+city+"%", models.LoungeVerificationApproved)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges by city: %w", err)
	}

	return lounges, nil
}

// UpdateLounge updates lounge information
func (r *LoungeRepository) UpdateLounge(id uuid.UUID, req CreateLoungeRequest) error {
	// Convert arrays to JSON
	photosJSON, err := json.Marshal(req.LoungePhotos)
	if err != nil {
		return fmt.Errorf("failed to marshal lounge photos: %w", err)
	}

	facilitiesJSON, err := json.Marshal(req.Facilities)
	if err != nil {
		return fmt.Errorf("failed to marshal facilities: %w", err)
	}

	var operatingHoursJSON []byte
	if req.OperatingHours != nil {
		operatingHoursJSON, err = json.Marshal(req.OperatingHours)
		if err != nil {
			return fmt.Errorf("failed to marshal operating hours: %w", err)
		}
	}

	query := `
		UPDATE lounges 
		SET 
			lounge_name = $1,
			business_license_number = $2,
			full_address = $3,
			city = $4,
			state = $5,
			postal_code = $6,
			latitude = $7,
			longitude = $8,
			contact_person_name = $9,
			business_email = $10,
			business_phone = $11,
			lounge_photos = $12,
			facilities = $13,
			operating_hours = $14,
			description = $15,
			updated_at = NOW()
		WHERE id = $16
	`

	result, err := r.db.Exec(
		query,
		req.LoungeName,
		req.BusinessLicense,
		req.FullAddress,
		req.City,
		req.State,
		req.PostalCode,
		req.Latitude,
		req.Longitude,
		req.ContactPersonName,
		req.BusinessEmail,
		req.BusinessPhone,
		photosJSON,
		facilitiesJSON,
		operatingHoursJSON,
		req.Description,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update lounge: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lounge not found")
	}

	return nil
}

// UpdateVerificationStatus updates the lounge verification status (for admin)
func (r *LoungeRepository) UpdateVerificationStatus(
	id uuid.UUID,
	status string,
	notes *string,
	verifiedBy uuid.UUID,
) error {
	query := `
		UPDATE lounges 
		SET 
			verification_status = $1,
			verification_notes = $2,
			verified_at = NOW(),
			verified_by = $3,
			updated_at = NOW()
		WHERE id = $4
	`

	_, err := r.db.Exec(query, status, notes, verifiedBy, id)
	if err != nil {
		return fmt.Errorf("failed to update verification status: %w", err)
	}

	return nil
}

// DeleteLounge soft deletes a lounge (sets is_active to false)
func (r *LoungeRepository) DeleteLounge(id uuid.UUID) error {
	query := `
		UPDATE lounges 
		SET 
			is_active = false,
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete lounge: %w", err)
	}

	return nil
}

// GetPendingLounges retrieves all lounges pending admin approval
func (r *LoungeRepository) GetPendingLounges() ([]*models.Lounge, error) {
	var lounges []*models.Lounge

	query := `
		SELECT * FROM lounges 
		WHERE verification_status = $1
		ORDER BY created_at ASC
	`

	err := r.db.Select(&lounges, query, models.LoungeVerificationPending)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending lounges: %w", err)
	}

	return lounges, nil
}

// CountLoungesByOwner counts the number of lounges for an owner
func (r *LoungeRepository) CountLoungesByOwner(ownerID uuid.UUID) (int, error) {
	var count int

	query := `
		SELECT COUNT(*) FROM lounges 
		WHERE lounge_owner_id = $1 AND is_active = true
	`

	err := r.db.QueryRow(query, ownerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count lounges: %w", err)
	}

	return count, nil
}
