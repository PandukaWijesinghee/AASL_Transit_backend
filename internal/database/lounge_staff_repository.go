package database

import (
	"database/sql"
	"fmt"
	"time"

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

// AddStaffToLounge adds a staff member to a lounge (owner invites staff)
func (r *LoungeStaffRepository) AddStaffToLounge(
	loungeID uuid.UUID,
	userID uuid.UUID,
	employmentStatus string,
) (*models.LoungeStaff, error) {
	staff := &models.LoungeStaff{
		ID:               uuid.New(),
		LoungeID:         loungeID,
		UserID:           userID,
		ProfileCompleted: false,
		EmploymentStatus: employmentStatus,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	query := `
		INSERT INTO lounge_staff (
			id, lounge_id, user_id, profile_completed,
			employment_status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowx(
		query,
		staff.ID,
		loungeID,
		userID,
		staff.ProfileCompleted,
		staff.EmploymentStatus,
		staff.CreatedAt,
		staff.UpdatedAt,
	).Scan(&staff.ID, &staff.CreatedAt, &staff.UpdatedAt)

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

// GetStaffByUserID retrieves a staff member by user_id
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

// UpdateStaffProfile updates staff profile information when they complete registration
func (r *LoungeStaffRepository) UpdateStaffProfile(
	userID uuid.UUID,
	fullName string,
	nicNumber string,
	email *string,
) error {
	query := `
		UPDATE lounge_staff 
		SET 
			full_name = $1,
			nic_number = $2,
			email = $3,
			profile_completed = true,
			updated_at = NOW()
		WHERE user_id = $4
	`

	var emailValue interface{}
	if email != nil && *email != "" {
		emailValue = *email
	} else {
		emailValue = nil
	}

	result, err := r.db.Exec(query, fullName, nicNumber, emailValue, userID)
	if err != nil {
		return fmt.Errorf("failed to update staff profile: %w", err)
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

// GetStaffWithUserDetails retrieves staff with user phone via JOIN
func (r *LoungeStaffRepository) GetStaffWithUserDetails(staffID uuid.UUID) (map[string]interface{}, error) {
	query := `
		SELECT 
			ls.*,
			u.phone as user_phone
		FROM lounge_staff ls
		JOIN users u ON ls.user_id = u.id
		WHERE ls.id = $1
	`
	
	result := make(map[string]interface{})
	err := r.db.QueryRowx(query, staffID).MapScan(result)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get staff with user details: %w", err)
	}
	
	return result, nil
}
