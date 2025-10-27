package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// UserRepository handles user database operations
type UserRepository struct {
	db DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

// CreateUser creates a new user in the database with default passenger role
func (r *UserRepository) CreateUser(phone string) (*models.User, error) {
	user := &models.User{
		ID:               uuid.New(),
		Phone:            phone,
		Roles:            []string{"passenger"}, // Default role
		Status:           "active",
		ProfileCompleted: false,
		PhoneVerified:    true, // Verified via OTP
		EmailVerified:    false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	query := `
		INSERT INTO users (
			id, phone, roles, status, 
			profile_completed, phone_verified, email_verified,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4::user_status, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(
		query,
		user.ID,
		user.Phone,
		pq.Array(user.Roles),
		user.Status,
		user.ProfileCompleted,
		user.PhoneVerified,
		user.EmailVerified,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// CreateUserWithoutRole creates a new user without assigning any role (for staff registration)
func (r *UserRepository) CreateUserWithoutRole(phone string) (*models.User, error) {
	user := &models.User{
		ID:               uuid.New(),
		Phone:            phone,
		Roles:            []string{}, // Empty roles array - role will be assigned during staff registration
		Status:           "active",
		ProfileCompleted: false,
		PhoneVerified:    true, // Verified via OTP
		EmailVerified:    false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	query := `
		INSERT INTO users (
			id, phone, roles, status, 
			profile_completed, phone_verified, email_verified,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4::user_status, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(
		query,
		user.ID,
		user.Phone,
		pq.Array(user.Roles),
		user.Status,
		user.ProfileCompleted,
		user.PhoneVerified,
		user.EmailVerified,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user without role: %w", err)
	}

	return user, nil
}

// CreateUserWithRole creates a new user with a specific role
func (r *UserRepository) CreateUserWithRole(phone string, role string) (*models.User, error) {
	// Validate role
	validRoles := map[string]bool{
		"passenger": true,
		"driver":    true,
		"conductor": true,
		"admin":     true,
	}

	if !validRoles[role] {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	user := &models.User{
		ID:               uuid.New(),
		Phone:            phone,
		Roles:            []string{role},
		Status:           "active",
		ProfileCompleted: false,
		PhoneVerified:    true, // Verified via OTP
		EmailVerified:    false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	query := `
		INSERT INTO users (
			id, phone, roles, status, 
			profile_completed, phone_verified, email_verified,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4::user_status, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(
		query,
		user.ID,
		user.Phone,
		pq.Array(user.Roles),
		user.Status,
		user.ProfileCompleted,
		user.PhoneVerified,
		user.EmailVerified,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user with role: %w", err)
	}

	return user, nil
}

// HasRole checks if a user has a specific role
func (r *UserRepository) HasRole(user *models.User, role string) bool {
	for _, r := range user.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// GetUserByPhone retrieves a user by phone number
func (r *UserRepository) GetUserByPhone(phone string) (*models.User, error) {
	var user models.User

	query := `
		SELECT id, phone, email, first_name, last_name, nic,
		       date_of_birth, address, city, postal_code, roles,
		       profile_photo_url, profile_completed, status,
		       phone_verified, email_verified, last_login_at,
		       metadata, created_at, updated_at
		FROM users
		WHERE phone = $1
	`

	err := r.db.Get(&user, query, phone)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(id uuid.UUID) (*models.User, error) {
	var user models.User

	query := `
		SELECT id, phone, email, first_name, last_name, nic,
		       date_of_birth, address, city, postal_code, roles,
		       profile_photo_url, profile_completed, status,
		       phone_verified, email_verified, last_login_at,
		       metadata, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	err := r.db.Get(&user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

// UpdateProfile updates user profile information
func (r *UserRepository) UpdateProfile(id uuid.UUID, firstName, lastName, email, address, city, postalCode string) error {
	query := `
		UPDATE users
		SET first_name = $1, 
		    last_name = $2,
		    email = $3, 
		    address = $4,
		    city = $5,
		    postal_code = $6,
		    updated_at = $7
		WHERE id = $8
	`

	result, err := r.db.Exec(query, firstName, lastName, email, address, city, postalCode, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateUserNames updates only first_name and last_name (without touching email or other fields)
func (r *UserRepository) UpdateUserNames(id uuid.UUID, firstName, lastName string) error {
	query := `
		UPDATE users
		SET first_name = $1,
		    last_name = $2,
		    updated_at = $3
		WHERE id = $4
	`

	result, err := r.db.Exec(query, firstName, lastName, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update user names: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// IsProfileComplete checks if user profile is complete
func (r *UserRepository) IsProfileComplete(id uuid.UUID) (bool, error) {
	var profileCompleted bool

	query := `
		SELECT profile_completed
		FROM users
		WHERE id = $1
	`

	err := r.db.QueryRow(query, id).Scan(&profileCompleted)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("user not found")
		}
		return false, fmt.Errorf("failed to check profile completion: %w", err)
	}

	return profileCompleted, nil
}

// UpdateProfileCompletion updates the profile completion status
func (r *UserRepository) UpdateProfileCompletion(id uuid.UUID) error {
	// Check if all required fields are filled
	var firstName, lastName, email, address sql.NullString

	query := `
		SELECT first_name, last_name, email, address
		FROM users
		WHERE id = $1
	`

	err := r.db.QueryRow(query, id).Scan(&firstName, &lastName, &email, &address)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user not found")
		}
		return fmt.Errorf("failed to get user fields: %w", err)
	}

	// Profile is complete if required fields are all filled
	isComplete := firstName.Valid && firstName.String != "" &&
		lastName.Valid && lastName.String != "" &&
		email.Valid && email.String != "" &&
		address.Valid && address.String != ""

	updateQuery := `
		UPDATE users
		SET profile_completed = $1,
		    updated_at = $2
		WHERE id = $3
	`

	_, err = r.db.Exec(updateQuery, isComplete, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update profile completion: %w", err)
	}

	return nil
}

// SetProfileCompleted directly sets the profile_completed status
// Used by bus owner onboarding to sync users.profile_completed with bus_owners.profile_completed
func (r *UserRepository) SetProfileCompleted(id uuid.UUID, completed bool) error {
	query := `
		UPDATE users
		SET profile_completed = $1,
		    updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, completed, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to set profile completion: %w", err)
	}

	return nil
}

// GetOrCreateUser gets an existing user or creates a new one
func (r *UserRepository) GetOrCreateUser(phone string) (*models.User, bool, error) {
	// Try to get existing user
	user, err := r.GetUserByPhone(phone)
	if err != nil {
		return nil, false, err
	}

	// If user exists, return it
	if user != nil {
		return user, false, nil
	}

	// Create new user
	user, err = r.CreateUser(phone)
	if err != nil {
		return nil, false, err
	}

	return user, true, nil
}

// UpdateUserStatus updates user status
func (r *UserRepository) UpdateUserStatus(id uuid.UUID, status string) error {
	// Validate status
	validStatuses := map[string]bool{
		"active":    true,
		"inactive":  true,
		"suspended": true,
		"banned":    true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	query := `
		UPDATE users
		SET status = $1,
		    updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.Exec(query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// AddUserRole adds a role to user
func (r *UserRepository) AddUserRole(id uuid.UUID, role string) error {
	// Validate role
	validRoles := map[string]bool{
		"passenger":  true,
		"driver":     true,
		"conductor":  true,
		"bus_owner":  true, // Added for bus owner onboarding
		"admin":      true,
	}

	if !validRoles[role] {
		return fmt.Errorf("invalid role: %s", role)
	}

	query := `
		UPDATE users
		SET roles = array_append(roles, $1),
		    updated_at = $2
		WHERE id = $3
		  AND NOT ($1 = ANY(roles))
	`

	_, err := r.db.Exec(query, role, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to add user role: %w", err)
	}

	return nil
}

// RemoveUserRole removes a role from user
func (r *UserRepository) RemoveUserRole(id uuid.UUID, role string) error {
	query := `
		UPDATE users
		SET roles = array_remove(roles, $1),
		    updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, role, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to remove user role: %w", err)
	}

	return nil
}

// ListUsers retrieves all users with pagination
func (r *UserRepository) ListUsers(limit, offset int) ([]*models.User, error) {
	var users []*models.User

	query := `
		SELECT id, phone, email, first_name, last_name, nic,
		       date_of_birth, address, city, postal_code, roles,
		       profile_photo_url, profile_completed, status,
		       phone_verified, email_verified, last_login_at,
		       metadata, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	err := r.db.Select(&users, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// CountUsers returns the total number of users
func (r *UserRepository) CountUsers() (int, error) {
	var count int

	query := `SELECT COUNT(*) FROM users`

	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}
