package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// AdminUserRepository handles admin user database operations
type AdminUserRepository struct {
	db DB
}

// NewAdminUserRepository creates a new admin user repository
func NewAdminUserRepository(db DB) *AdminUserRepository {
	return &AdminUserRepository{db: db}
}

// GetByEmail retrieves an admin user by email
func (r *AdminUserRepository) GetByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	query := `
		SELECT id, email, password_hash, full_name, is_active, last_login_at,
		       created_at, updated_at, created_by
		FROM admin_users
		WHERE email = $1
	`

	var admin models.AdminUser
	err := r.db.QueryRow(query, email).Scan(
		&admin.ID, &admin.Email, &admin.PasswordHash, &admin.FullName, &admin.IsActive,
		&admin.LastLoginAt, &admin.CreatedAt, &admin.UpdatedAt, &admin.CreatedBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin user not found")
		}
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	return &admin, nil
}

// GetByID retrieves an admin user by ID
func (r *AdminUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error) {
	query := `
		SELECT id, email, password_hash, full_name, is_active, last_login_at,
		       created_at, updated_at, created_by
		FROM admin_users
		WHERE id = $1
	`

	var admin models.AdminUser
	err := r.db.QueryRow(query, id).Scan(
		&admin.ID, &admin.Email, &admin.PasswordHash, &admin.FullName, &admin.IsActive,
		&admin.LastLoginAt, &admin.CreatedAt, &admin.UpdatedAt, &admin.CreatedBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin user not found")
		}
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	return &admin, nil
}

// Create creates a new admin user
func (r *AdminUserRepository) Create(ctx context.Context, admin *models.AdminUser) error {
	// Generate UUID if not provided
	if admin.ID == uuid.Nil {
		admin.ID = uuid.New()
	}

	query := `
		INSERT INTO admin_users (id, email, password_hash, full_name, is_active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(query,
		admin.ID,
		admin.Email,
		admin.PasswordHash,
		admin.FullName,
		admin.IsActive,
		admin.CreatedBy,
	).Scan(&admin.CreatedAt, &admin.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp
func (r *AdminUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE admin_users
		SET last_login_at = $1, updated_at = $1
		WHERE id = $2
	`

	now := time.Now()
	_, err := r.db.Exec(query, now, id)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// UpdatePassword updates the admin user's password
func (r *AdminUserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := `
		UPDATE admin_users
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, passwordHash, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// List retrieves all admin users
func (r *AdminUserRepository) List(ctx context.Context) ([]*models.AdminUser, error) {
	query := `
		SELECT id, email, password_hash, full_name, is_active, last_login_at,
		       created_at, updated_at, created_by
		FROM admin_users
		ORDER BY created_at DESC
	`

	var admins []*models.AdminUser
	err := r.db.Select(&admins, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list admin users: %w", err)
	}

	return admins, nil
}

// UpdateActiveStatus updates the active status of an admin user
func (r *AdminUserRepository) UpdateActiveStatus(ctx context.Context, id uuid.UUID, isActive bool) error {
	query := `
		UPDATE admin_users
		SET is_active = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, isActive, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update active status: %w", err)
	}

	return nil
}
