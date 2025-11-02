package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeStaffRepository handles database operations for lounge staff
type LoungeStaffRepository struct {
	db *sqlx.DB
}

// NewLoungeStaffRepository creates a new lounge staff repository
func NewLoungeStaffRepository(db *sqlx.DB) *LoungeStaffRepository {
	return &LoungeStaffRepository{db: db}
}

// AddStaffToLounge adds a staff member to a lounge (Step 4 - optional)
func (r *LoungeStaffRepository) AddStaffToLounge(
	loungeID uuid.UUID,
	phoneNumber string,
	fullName string,
	nicNumber string,
	permissionType string,
) (*models.LoungeStaff, error) {
	staff := &models.LoungeStaff{
		ID:               uuid.New(),
		LoungeID:         loungeID,
		PhoneNumber:      phoneNumber,
		FullName:         sql.NullString{String: fullName, Valid: fullName != ""},
		NICNumber:        sql.NullString{String: nicNumber, Valid: nicNumber != ""},
		PermissionType:   permissionType,
		EmploymentStatus: models.StaffStatusActive,
		HasRegistered:    false,
	}

	query := `
		INSERT INTO lounge_staff (
			id, lounge_id, phone_number, full_name, nic_number,
			permission_type, employment_status, has_registered,
			invited_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW(), NOW())
		RETURNING id, invited_at, created_at, updated_at
	`

	err := r.db.QueryRowx(
		query,
		staff.ID,
		loungeID,
		phoneNumber,
		fullName,
		nicNumber,
		permissionType,
		staff.EmploymentStatus,
		staff.HasRegistered,
	).Scan(&staff.ID, &staff.InvitedAt, &staff.CreatedAt, &staff.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to add staff: %w", err)
	}

	return staff, nil
}

// GetStaffByID retrieves a staff member by ID
func (r *LoungeStaffRepository) GetStaffByID(id uuid.UUID) (*models.LoungeStaff, error) {
	var staff models.LoungeStaff
	query := `SELECT * FROM lounge_staff WHERE id = $1`
	err := r.db.Get(&staff, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get staff: %w", err)
	}
	return &staff, nil
}

// GetStaffByLoungeID retrieves all staff for a specific lounge
func (r *LoungeStaffRepository) GetStaffByLoungeID(loungeID uuid.UUID) ([]models.LoungeStaff, error) {
	var staff []models.LoungeStaff
	query := `
		SELECT * FROM lounge_staff 
		WHERE lounge_id = $1 
		ORDER BY created_at DESC
	`
	err := r.db.Select(&staff, query, loungeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get staff: %w", err)
	}
	return staff, nil
}

// GetStaffByPhoneNumber retrieves a staff member by phone number
func (r *LoungeStaffRepository) GetStaffByPhoneNumber(phoneNumber string) (*models.LoungeStaff, error) {
	var staff models.LoungeStaff
	query := `SELECT * FROM lounge_staff WHERE phone_number = $1`
	err := r.db.Get(&staff, query, phoneNumber)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get staff: %w", err)
	}
	return &staff, nil
}

// GetStaffByUserID retrieves a staff member by user_id (after registration)
func (r *LoungeStaffRepository) GetStaffByUserID(userID uuid.UUID) (*models.LoungeStaff, error) {
	var staff models.LoungeStaff
	query := `SELECT * FROM lounge_staff WHERE user_id = $1`
	err := r.db.Get(&staff, query, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get staff: %w", err)
	}
	return &staff, nil
}

// UpdateStaffRegistration links staff to user account after they register
func (r *LoungeStaffRepository) UpdateStaffRegistration(
	phoneNumber string,
	userID uuid.UUID,
) error {
	query := `
		UPDATE lounge_staff 
		SET 
			user_id = $1,
			has_registered = true,
			registered_at = NOW(),
			updated_at = NOW()
		WHERE phone_number = $2 AND has_registered = false
	`

	result, err := r.db.Exec(query, userID, phoneNumber)
	if err != nil {
		return fmt.Errorf("failed to update staff registration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("staff not found or already registered")
	}

	return nil
}

// UpdateStaffDetails updates staff member information
func (r *LoungeStaffRepository) UpdateStaffDetails(
	id uuid.UUID,
	fullName string,
	nicNumber string,
	email *string,
	nicFrontURL *string,
	nicBackURL *string,
) error {
	query := `
		UPDATE lounge_staff 
		SET 
			full_name = $1,
			nic_number = $2,
			email = $3,
			nic_front_url = $4,
			nic_back_url = $5,
			updated_at = NOW()
		WHERE id = $6
	`

	result, err := r.db.Exec(
		query,
		fullName,
		nicNumber,
		email,
		nicFrontURL,
		nicBackURL,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update staff details: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("staff not found")
	}

	return nil
}

// UpdateStaffPermission updates staff permission type (admin/staff)
func (r *LoungeStaffRepository) UpdateStaffPermission(
	id uuid.UUID,
	permissionType string,
) error {
	query := `
		UPDATE lounge_staff 
		SET 
			permission_type = $1,
			updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(query, permissionType, id)
	if err != nil {
		return fmt.Errorf("failed to update staff permission: %w", err)
	}

	return nil
}

// UpdateStaffEmploymentStatus updates staff employment status
func (r *LoungeStaffRepository) UpdateStaffEmploymentStatus(
	id uuid.UUID,
	status string,
) error {
	query := `
		UPDATE lounge_staff 
		SET 
			employment_status = $1,
			updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update staff status: %w", err)
	}

	return nil
}

// RemoveStaff deletes a staff member
func (r *LoungeStaffRepository) RemoveStaff(id uuid.UUID) error {
	query := `DELETE FROM lounge_staff WHERE id = $1`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to remove staff: %w", err)
	}
	return nil
}

// GetActiveStaffByLoungeID retrieves all active staff for a lounge
func (r *LoungeStaffRepository) GetActiveStaffByLoungeID(loungeID uuid.UUID) ([]models.LoungeStaff, error) {
	var staff []models.LoungeStaff
	query := `
		SELECT * FROM lounge_staff 
		WHERE lounge_id = $1 AND employment_status = 'active'
		ORDER BY created_at DESC
	`
	err := r.db.Select(&staff, query, loungeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active staff: %w", err)
	}
	return staff, nil
}
