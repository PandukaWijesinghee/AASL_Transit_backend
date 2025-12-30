package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeOwnerRepository handles database operations for lounge owners
type LoungeOwnerRepository struct {
	db *sqlx.DB
}

// NewLoungeOwnerRepository creates a new lounge owner repository
func NewLoungeOwnerRepository(db *sqlx.DB) *LoungeOwnerRepository {
	return &LoungeOwnerRepository{db: db}
}

// CreateLoungeOwner creates a new lounge owner record after OTP verification (Step 0)
func (r *LoungeOwnerRepository) CreateLoungeOwner(userID uuid.UUID) (*models.LoungeOwner, error) {
	loungeOwner := &models.LoungeOwner{
		ID:                 uuid.New(),
		UserID:             userID,
		RegistrationStep:   models.RegStepPhoneVerified,
		ProfileCompleted:   false,
		VerificationStatus: "pending",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	query := `
		INSERT INTO lounge_owners (
			id, user_id, registration_step, profile_completed, 
			verification_status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowx(
		query,
		loungeOwner.ID,
		loungeOwner.UserID,
		loungeOwner.RegistrationStep,
		loungeOwner.ProfileCompleted,
		loungeOwner.VerificationStatus,
		loungeOwner.CreatedAt,
		loungeOwner.UpdatedAt,
	).Scan(&loungeOwner.ID, &loungeOwner.CreatedAt, &loungeOwner.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create lounge owner: %w", err)
	}

	return loungeOwner, nil
}

// GetLoungeOwnerByUserID retrieves a lounge owner by user ID
func (r *LoungeOwnerRepository) GetLoungeOwnerByUserID(userID uuid.UUID) (*models.LoungeOwner, error) {
	var owner models.LoungeOwner
	query := `
		SELECT 
			id, user_id, business_name, business_license, 
			manager_full_name, manager_nic_number, manager_email,
			registration_step, profile_completed,
			verification_status, verification_notes, verified_at, verified_by,
			created_at, updated_at
		FROM lounge_owners 
		WHERE user_id = $1
	`
	err := r.db.Get(&owner, query, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lounge owner: %w", err)
	}
	return &owner, nil
}

// GetLoungeOwnerByID retrieves a lounge owner by ID
func (r *LoungeOwnerRepository) GetLoungeOwnerByID(id uuid.UUID) (*models.LoungeOwner, error) {
	var owner models.LoungeOwner
	query := `
		SELECT 
			id, user_id, business_name, business_license, 
			manager_full_name, manager_nic_number, manager_email,
			registration_step, profile_completed,
			verification_status, verification_notes, verified_at, verified_by,
			created_at, updated_at
		FROM lounge_owners 
		WHERE id = $1
	`
	err := r.db.Get(&owner, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lounge owner: %w", err)
	}
	return &owner, nil
}

// UpdateBusinessAndManagerInfo updates business and manager information (Step 1)
func (r *LoungeOwnerRepository) UpdateBusinessAndManagerInfo(
	userID uuid.UUID,
	businessName string,
	businessLicense string,
	managerFullName string,
	managerNICNumber string,
	managerEmail *string,
) error {
	query := `
		UPDATE lounge_owners 
		SET 
			business_name = $1,
			business_license = $2,
			manager_full_name = $3,
			manager_nic_number = $4,
			manager_email = $5,
			registration_step = $6,
			updated_at = NOW()
		WHERE user_id = $7
	`

	var emailValue interface{}
	if managerEmail != nil && *managerEmail != "" {
		emailValue = *managerEmail
	} else {
		emailValue = nil
	}

	result, err := r.db.Exec(
		query,
		businessName,
		businessLicense,
		managerFullName,
		managerNICNumber,
		emailValue,
		models.RegStepBusinessInfo,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to update business and manager info: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lounge owner not found")
	}

	return nil
}

// UpdateBusinessAndManagerInfoWithNIC updates business and manager information (Step 1 - New Flow)
// Note: NIC images are now stored in Supabase only, not in database
func (r *LoungeOwnerRepository) UpdateBusinessAndManagerInfoWithNIC(
	userID uuid.UUID,
	businessName string,
	businessLicense string,
	managerFullName string,
	managerNICNumber string,
	managerEmail *string,
	managerNICFrontURL *string,
	managerNICBackURL *string,
) error {
	// Handle empty business license as NULL to avoid unique constraint issues
	var businessLicenseValue interface{}
	if businessLicense == "" {
		businessLicenseValue = nil
	} else {
		businessLicenseValue = businessLicense
	}

	query := `
		UPDATE lounge_owners
		SET
			business_name = $1,
			business_license = $2,
			manager_full_name = $3,
			manager_nic_number = $4,
			manager_email = $5,
			registration_step = $6,
			updated_at = NOW()
		WHERE user_id = $7
	`

	var emailValue interface{}
	if managerEmail != nil && *managerEmail != "" {
		emailValue = *managerEmail
	} else {
		emailValue = nil
	}

	result, err := r.db.Exec(
		query,
		businessName,
		businessLicenseValue,
		managerFullName,
		managerNICNumber,
		emailValue,
		models.RegStepBusinessInfo,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to update business and manager info with NIC: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lounge owner not found")
	}

	return nil
}

// UpdateManagerNICImages updates manager NIC image URLs (Step 2)
// DEPRECATED: NIC verification step removed. This function only updates URLs without changing registration step.
func (r *LoungeOwnerRepository) UpdateManagerNICImages(
	userID uuid.UUID,
	frontImageURL string,
	backImageURL string,
) error {
	query := `
		UPDATE lounge_owners
		SET
			manager_nic_front_url = $1,
			manager_nic_back_url = $2,
			updated_at = NOW()
		WHERE user_id = $3
	`

	result, err := r.db.Exec(
		query,
		frontImageURL,
		backImageURL,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to update manager NIC images: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lounge owner not found")
	}

	return nil
}

// UpdateRegistrationStep updates the registration step
func (r *LoungeOwnerRepository) UpdateRegistrationStep(userID uuid.UUID, step string) error {
	query := `
		UPDATE lounge_owners 
		SET 
			registration_step = $1,
			updated_at = NOW()
		WHERE user_id = $2
	`

	_, err := r.db.Exec(query, step, userID)
	if err != nil {
		return fmt.Errorf("failed to update registration step: %w", err)
	}

	return nil
}

// CompleteRegistration marks the registration as completed
func (r *LoungeOwnerRepository) CompleteRegistration(userID uuid.UUID) error {
	query := `
		UPDATE lounge_owners 
		SET 
			registration_step = $1,
			profile_completed = true,
			updated_at = NOW()
		WHERE user_id = $2
	`

	_, err := r.db.Exec(query, models.RegStepCompleted, userID)
	if err != nil {
		return fmt.Errorf("failed to complete registration: %w", err)
	}

	return nil
}

// GetRegistrationProgress returns the current registration step and completion status
func (r *LoungeOwnerRepository) GetRegistrationProgress(userID uuid.UUID) (string, bool, error) {
	var step string
	var completed bool

	query := `
		SELECT registration_step, profile_completed 
		FROM lounge_owners 
		WHERE user_id = $1
	`

	err := r.db.QueryRow(query, userID).Scan(&step, &completed)
	if err != nil {
		return "", false, fmt.Errorf("failed to get registration progress: %w", err)
	}

	return step, completed, nil
}

// UpdateVerificationStatus updates the verification status (for admin approval)
func (r *LoungeOwnerRepository) UpdateVerificationStatus(id uuid.UUID, status string, notes *string, verifiedBy uuid.UUID) error {
	query := `
		UPDATE lounge_owners 
		SET 
			verification_status = $1,
			verification_notes = $2,
			verified_at = NOW(),
			verified_by = $3,
			updated_at = NOW()
		WHERE id = $4
	`

	var notesValue interface{}
	if notes != nil {
		notesValue = *notes
	} else {
		notesValue = nil
	}

	_, err := r.db.Exec(query, status, notesValue, verifiedBy, id)
	if err != nil {
		return fmt.Errorf("failed to update verification status: %w", err)
	}

	return nil
}

// GetPendingLoungeOwners retrieves all lounge owners pending verification
func (r *LoungeOwnerRepository) GetPendingLoungeOwners(limit int, offset int) ([]models.LoungeOwner, error) {
	var owners []models.LoungeOwner

	query := `
		SELECT * FROM lounge_owners 
		WHERE verification_status = 'pending' AND profile_completed = true
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	err := r.db.Select(&owners, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending lounge owners: %w", err)
	}

	return owners, nil
}

// GetLoungeCount returns the number of lounges for a lounge owner
func (r *LoungeOwnerRepository) GetLoungeCount(ownerID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM lounges WHERE lounge_owner_id = $1`
	err := r.db.Get(&count, query, ownerID)
	if err != nil {
		return 0, fmt.Errorf("failed to get lounge count: %w", err)
	}
	return count, nil
}

// GetStaffCount returns the number of staff across all lounges for a lounge owner
func (r *LoungeOwnerRepository) GetStaffCount(ownerID uuid.UUID) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) 
		FROM lounge_staff ls
		JOIN lounges l ON ls.lounge_id = l.id
		WHERE l.lounge_owner_id = $1
	`
	err := r.db.Get(&count, query, ownerID)
	if err != nil {
		return 0, fmt.Errorf("failed to get staff count: %w", err)
	}
	return count, nil
}
