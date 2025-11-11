package models

import (
	"time"

	"github.com/google/uuid"
)

// AdminUser represents an admin dashboard user
type AdminUser struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"` // Never expose password hash in JSON
	FullName     string     `json:"full_name" db:"full_name"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
}

// AdminLoginRequest represents the login request payload
type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// AdminLoginResponse represents the login response
type AdminLoginResponse struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresIn    int64      `json:"expires_in"`
	AdminUser    *AdminUser `json:"admin_user"`
}

// AdminRefreshRequest represents the token refresh request
type AdminRefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// AdminChangePasswordRequest represents the change password request
type AdminChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// AdminCreateRequest represents the request to create a new admin user
type AdminCreateRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
}
