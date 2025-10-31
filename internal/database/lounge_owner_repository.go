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

// CreateLoungeOwner creates a new lounge owner record after OTP verification
func (r *LoungeOwnerRepository) CreateLoungeOwner(userID uuid.UUID) (*models.LoungeOwner, error) {
	loungeOwner := &models.LoungeOwner{
		ID:                 uuid.New(),
		UserID:             userID,
		RegistrationStep:   models.RegStepPhoneVerified,
		VerificationStatus: "pending",
		NICOCRAttempts:     0,
		TotalLounges:       0,
		ProfileCompleted:   false,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	query := `
		INSERT INTO lounge_owners (
			id, user_id, registration_step, verification_status, 
			nic_ocr_attempts, total_lounges, profile_completed, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowx(
		query,
		loungeOwner.ID,
		loungeOwner.UserID,
		loungeOwner.RegistrationStep,
		loungeOwner.VerificationStatus,
		loungeOwner.NICOCRAttempts,
		loungeOwner.TotalLounges,
		loungeOwner.ProfileCompleted,
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

	query := `SELECT * FROM lounge_owners WHERE user_id = $1`

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

	query := `SELECT * FROM lounge_owners WHERE id = $1`

	err := r.db.Get(&owner, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lounge owner: %w", err)
	}

	return &owner, nil
}

// UpdatePersonalInfo updates lounge owner personal information (Step 1)
func (r *LoungeOwnerRepository) UpdatePersonalInfo(
	userID uuid.UUID,
	fullName string,
	nicNumber string,
	email *string,
) error {
	query := `
		UPDATE lounge_owners 
		SET 
			full_name = $1,
			nic_number = $2,
			business_email = $3,
			registration_step = $4,
			updated_at = NOW()
		WHERE user_id = $5
	`

	var emailValue interface{}
	if email != nil && *email != "" {
		emailValue = *email
	} else {
		emailValue = nil
	}

	result, err := r.db.Exec(
		query,
		fullName,
		nicNumber,
		emailValue,
		models.RegStepPersonalInfo,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to update personal info: %w", err)
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

// UpdateNICImages updates NIC image URLs (Step 2)
func (r *LoungeOwnerRepository) UpdateNICImages(
	userID uuid.UUID,
	frontImageURL string,
	backImageURL string,
) error {
	query := `
		UPDATE lounge_owners 
		SET 
			nic_front_image_url = $1,
			nic_back_image_url = $2,
			registration_step = $3,
			updated_at = NOW()
		WHERE user_id = $4
	`

	result, err := r.db.Exec(
		query,
		frontImageURL,
		backImageURL,
		models.RegStepNICUploaded,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to update NIC images: %w", err)
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

// IncrementOCRAttempts increments the OCR attempt counter
func (r *LoungeOwnerRepository) IncrementOCRAttempts(userID uuid.UUID) error {
	query := `
		UPDATE lounge_owners 
		SET 
			nic_ocr_attempts = nic_ocr_attempts + 1,
			last_ocr_attempt_at = NOW(),
			updated_at = NOW()
		WHERE user_id = $1
	`

	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to increment OCR attempts: %w", err)
	}

	return nil
}

// SetOCRBlock blocks OCR attempts for 24 hours
func (r *LoungeOwnerRepository) SetOCRBlock(userID uuid.UUID) error {
	blockUntil := time.Now().Add(models.OCRBlockDuration)

	query := `
		UPDATE lounge_owners 
		SET 
			ocr_blocked_until = $1,
			updated_at = NOW()
		WHERE user_id = $2
	`

	_, err := r.db.Exec(query, blockUntil, userID)
	if err != nil {
		return fmt.Errorf("failed to set OCR block: %w", err)
	}

	return nil
}

// IsOCRBlocked checks if OCR attempts are currently blocked
func (r *LoungeOwnerRepository) IsOCRBlocked(userID uuid.UUID) (bool, time.Time, error) {
	var blockedUntil sql.NullTime

	query := `
		SELECT ocr_blocked_until 
		FROM lounge_owners 
		WHERE user_id = $1
	`

	err := r.db.QueryRow(query, userID).Scan(&blockedUntil)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to check OCR block: %w", err)
	}

	if !blockedUntil.Valid {
		return false, time.Time{}, nil
	}

	if time.Now().Before(blockedUntil.Time) {
		return true, blockedUntil.Time, nil
	}

	return false, time.Time{}, nil
}

// GetOCRAttempts returns the number of OCR attempts
func (r *LoungeOwnerRepository) GetOCRAttempts(userID uuid.UUID) (int, error) {
	var attempts int

	query := `
		SELECT nic_ocr_attempts 
		FROM lounge_owners 
		WHERE user_id = $1
	`

	err := r.db.QueryRow(query, userID).Scan(&attempts)
	if err != nil {
		return 0, fmt.Errorf("failed to get OCR attempts: %w", err)
	}

	return attempts, nil
}

// ResetOCRAttempts resets OCR attempts counter (called after 24h block expires)
func (r *LoungeOwnerRepository) ResetOCRAttempts(userID uuid.UUID) error {
	query := `
		UPDATE lounge_owners 
		SET 
			nic_ocr_attempts = 0,
			ocr_blocked_until = NULL,
			updated_at = NOW()
		WHERE user_id = $1
	`

	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to reset OCR attempts: %w", err)
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

// UpdateVerificationStatus updates the verification status (for admin approval)
func (r *LoungeOwnerRepository) UpdateVerificationStatus(id uuid.UUID, status string, notes *string) error {
	query := `
		UPDATE lounge_owners 
		SET 
			verification_status = $1,
			updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update verification status: %w", err)
	}

	return nil
}

// IncrementTotalLounges increments the total lounges count
func (r *LoungeOwnerRepository) IncrementTotalLounges(userID uuid.UUID) error {
	query := `
		UPDATE lounge_owners 
		SET 
			total_lounges = total_lounges + 1,
			updated_at = NOW()
		WHERE user_id = $1
	`

	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to increment total lounges: %w", err)
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
