package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

// AdminAuthService handles admin authentication business logic
type AdminAuthService struct {
	adminRepo            *database.AdminUserRepository
	refreshTokenRepo     *database.RefreshTokenRepository
	jwtService           *jwt.Service
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
}

// NewAdminAuthService creates a new admin auth service
func NewAdminAuthService(
	adminRepo *database.AdminUserRepository,
	refreshTokenRepo *database.RefreshTokenRepository,
	jwtService *jwt.Service,
	accessTokenDuration time.Duration,
	refreshTokenDuration time.Duration,
) *AdminAuthService {
	return &AdminAuthService{
		adminRepo:            adminRepo,
		refreshTokenRepo:     refreshTokenRepo,
		jwtService:           jwtService,
		accessTokenDuration:  accessTokenDuration,
		refreshTokenDuration: refreshTokenDuration,
	}
}

// Login authenticates an admin user and returns tokens
func (s *AdminAuthService) Login(ctx context.Context, email, password string) (*models.AdminLoginResponse, error) {
	// Get admin user by email
	admin, err := s.adminRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if admin is active
	if !admin.IsActive {
		return nil, fmt.Errorf("account is inactive")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Generate access token with admin role
	// Use email as "phone" since admin users don't have phone numbers
	accessToken, err := s.jwtService.GenerateAccessToken(admin.ID, admin.Email, []string{"admin"}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := s.jwtService.GenerateRefreshToken(admin.ID, admin.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token in database
	expiresAt := time.Now().Add(s.refreshTokenDuration)
	if err := s.refreshTokenRepo.StoreRefreshToken(
		admin.ID,
		refreshToken,
		"", // deviceID
		"", // deviceType
		"", // ipAddress
		"", // userAgent
		expiresAt,
	); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Update last login
	if err := s.adminRepo.UpdateLastLogin(ctx, admin.ID); err != nil {
		// Log error but don't fail the login
		fmt.Printf("Warning: failed to update last login for admin %s: %v\n", admin.ID, err)
	}

	return &models.AdminLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTokenDuration.Seconds()),
		AdminUser:    admin,
	}, nil
}

// RefreshToken generates a new access token from a refresh token
func (s *AdminAuthService) RefreshToken(ctx context.Context, refreshToken string) (*models.AdminLoginResponse, error) {
	// Validate refresh token
	claims, err := s.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Check if refresh token is revoked or expired
	storedToken, err := s.refreshTokenRepo.GetRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found")
	}

	if storedToken.Revoked {
		return nil, fmt.Errorf("refresh token has been revoked")
	}

	if time.Now().After(storedToken.ExpiresAt) {
		return nil, fmt.Errorf("refresh token has expired")
	}

	// Get admin user (UserID is already uuid.UUID in claims)
	admin, err := s.adminRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("admin user not found")
	}

	// Check if admin is still active
	if !admin.IsActive {
		return nil, fmt.Errorf("account is inactive")
	}

	// Generate new access token
	accessToken, err := s.jwtService.GenerateAccessToken(admin.ID, admin.Email, []string{"admin"}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Update last used timestamp
	if err := s.refreshTokenRepo.UpdateLastUsed(refreshToken); err != nil {
		fmt.Printf("Warning: failed to update last used for token: %v\n", err)
	}

	return &models.AdminLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTokenDuration.Seconds()),
		AdminUser:    admin,
	}, nil
}

// Logout revokes the refresh token
func (s *AdminAuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.refreshTokenRepo.RevokeToken(refreshToken)
}

// ChangePassword changes an admin user's password
func (s *AdminAuthService) ChangePassword(ctx context.Context, adminID uuid.UUID, oldPassword, newPassword string) error {
	// Get admin user
	admin, err := s.adminRepo.GetByID(ctx, adminID)
	if err != nil {
		return fmt.Errorf("admin user not found")
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("incorrect old password")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := s.adminRepo.UpdatePassword(ctx, adminID, string(hashedPassword)); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// CreateAdmin creates a new admin user
func (s *AdminAuthService) CreateAdmin(ctx context.Context, email, password, fullName string, createdBy uuid.UUID) (*models.AdminUser, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	admin := &models.AdminUser{
		Email:        email,
		PasswordHash: string(hashedPassword),
		FullName:     fullName,
		IsActive:     true,
		CreatedBy:    &createdBy,
	}

	if err := s.adminRepo.Create(ctx, admin); err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	return admin, nil
}

// GetAdminProfile retrieves admin user profile
func (s *AdminAuthService) GetAdminProfile(ctx context.Context, adminID uuid.UUID) (*models.AdminUser, error) {
	return s.adminRepo.GetByID(ctx, adminID)
}

// ListAdmins retrieves all admin users
func (s *AdminAuthService) ListAdmins(ctx context.Context) ([]*models.AdminUser, error) {
	return s.adminRepo.List(ctx)
}
