package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// AdminRefreshTokenRepository handles admin refresh token database operations
type AdminRefreshTokenRepository struct {
	db DB
}

// NewAdminRefreshTokenRepository creates a new admin refresh token repository
func NewAdminRefreshTokenRepository(db DB) *AdminRefreshTokenRepository {
	return &AdminRefreshTokenRepository{
		db: db,
	}
}

// hashToken creates a SHA-256 hash of the token for storage
func hashAdminToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// StoreRefreshToken stores an admin refresh token in the database
func (r *AdminRefreshTokenRepository) StoreRefreshToken(
	adminUserID uuid.UUID,
	token string,
	deviceID, deviceType, ipAddress, userAgent string,
	expiresAt time.Time,
) error {
	tokenHash := hashAdminToken(token)

	query := `
		INSERT INTO admin_refresh_tokens (
			admin_user_id, token_hash, device_id, device_type,
			ip_address, user_agent, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	var deviceIDVal, deviceTypeVal, ipVal, userAgentVal interface{}

	if deviceID != "" {
		deviceIDVal = deviceID
	}
	if deviceType != "" {
		deviceTypeVal = deviceType
	}
	if ipAddress != "" {
		ipVal = ipAddress
	}
	if userAgent != "" {
		userAgentVal = userAgent
	}

	_, err := r.db.Exec(
		query,
		adminUserID,
		tokenHash,
		deviceIDVal,
		deviceTypeVal,
		ipVal,
		userAgentVal,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store admin refresh token: %w", err)
	}

	return nil
}

// GetRefreshToken retrieves an admin refresh token by its hash
func (r *AdminRefreshTokenRepository) GetRefreshToken(token string) (*models.RefreshToken, error) {
	tokenHash := hashAdminToken(token)

	var refreshToken models.RefreshToken

	query := `
		SELECT id, admin_user_id as user_id, token_hash, device_id, device_type,
		       ip_address, user_agent, created_at, expires_at,
		       last_used_at, revoked, revoked_at
		FROM admin_refresh_tokens
		WHERE token_hash = $1
	`

	err := r.db.Get(&refreshToken, query, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Token not found
		}
		return nil, fmt.Errorf("failed to get admin refresh token: %w", err)
	}

	return &refreshToken, nil
}

// IsTokenRevoked checks if an admin refresh token is revoked
func (r *AdminRefreshTokenRepository) IsTokenRevoked(token string) (bool, error) {
	refreshToken, err := r.GetRefreshToken(token)
	if err != nil {
		return false, err
	}

	if refreshToken == nil {
		return true, nil // Token not found, consider it revoked
	}

	return refreshToken.Revoked, nil
}

// IsTokenExpired checks if an admin refresh token is expired
func (r *AdminRefreshTokenRepository) IsTokenExpired(token string) (bool, error) {
	refreshToken, err := r.GetRefreshToken(token)
	if err != nil {
		return false, err
	}

	if refreshToken == nil {
		return true, nil // Token not found, consider it expired
	}

	return refreshToken.ExpiresAt.Before(time.Now()), nil
}

// RevokeToken revokes a specific admin refresh token
func (r *AdminRefreshTokenRepository) RevokeToken(token string) error {
	tokenHash := hashAdminToken(token)

	query := `
		UPDATE admin_refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE token_hash = $2 AND revoked = FALSE
	`

	result, err := r.db.Exec(query, time.Now(), tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke admin token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("admin token not found or already revoked")
	}

	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for an admin user
func (r *AdminRefreshTokenRepository) RevokeAllUserTokens(adminUserID uuid.UUID) error {
	query := `
		UPDATE admin_refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE admin_user_id = $2 AND revoked = FALSE
	`

	_, err := r.db.Exec(query, time.Now(), adminUserID)
	if err != nil {
		return fmt.Errorf("failed to revoke all admin user tokens: %w", err)
	}

	return nil
}

// RevokeDeviceTokens revokes all tokens for a specific device
func (r *AdminRefreshTokenRepository) RevokeDeviceTokens(adminUserID uuid.UUID, deviceID string) error {
	query := `
		UPDATE admin_refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE admin_user_id = $2 AND device_id = $3 AND revoked = FALSE
	`

	_, err := r.db.Exec(query, time.Now(), adminUserID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to revoke admin device tokens: %w", err)
	}

	return nil
}

// UpdateLastUsed updates the last_used_at timestamp for an admin token
func (r *AdminRefreshTokenRepository) UpdateLastUsed(token string) error {
	tokenHash := hashAdminToken(token)

	query := `
		UPDATE admin_refresh_tokens
		SET last_used_at = $1
		WHERE token_hash = $2
	`

	_, err := r.db.Exec(query, time.Now(), tokenHash)
	if err != nil {
		return fmt.Errorf("failed to update admin token last used timestamp: %w", err)
	}

	return nil
}

// CleanupExpiredTokens removes expired admin refresh tokens
func (r *AdminRefreshTokenRepository) CleanupExpiredTokens() (int64, error) {
	query := `
		DELETE FROM admin_refresh_tokens
		WHERE expires_at < $1
	`

	result, err := r.db.Exec(query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired admin tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// CleanupRevokedTokens removes revoked admin tokens older than specified duration
func (r *AdminRefreshTokenRepository) CleanupRevokedTokens(olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `
		DELETE FROM admin_refresh_tokens
		WHERE revoked = TRUE AND revoked_at < $1
	`

	result, err := r.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup revoked admin tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// GetUserTokens retrieves all active tokens for an admin user
func (r *AdminRefreshTokenRepository) GetUserTokens(adminUserID uuid.UUID) ([]*models.RefreshToken, error) {
	var tokens []*models.RefreshToken

	query := `
		SELECT id, admin_user_id as user_id, token_hash, device_id, device_type,
		       ip_address, user_agent, created_at, expires_at,
		       last_used_at, revoked, revoked_at
		FROM admin_refresh_tokens
		WHERE admin_user_id = $1 AND revoked = FALSE AND expires_at > $2
		ORDER BY created_at DESC
	`

	err := r.db.Select(&tokens, query, adminUserID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get admin user tokens: %w", err)
	}

	return tokens, nil
}

// CountUserTokens counts active tokens for an admin user
func (r *AdminRefreshTokenRepository) CountUserTokens(adminUserID uuid.UUID) (int, error) {
	var count int

	query := `
		SELECT COUNT(*)
		FROM admin_refresh_tokens
		WHERE admin_user_id = $1 AND revoked = FALSE AND expires_at > $2
	`

	err := r.db.QueryRow(query, adminUserID, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count admin user tokens: %w", err)
	}

	return count, nil
}

// RevokeMostRecentToken revokes the most recent active token for an admin user
// This is useful when logout is called without a specific refresh token
func (r *AdminRefreshTokenRepository) RevokeMostRecentToken(adminUserID uuid.UUID) error {
	query := `
		UPDATE admin_refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE id = (
			SELECT id
			FROM admin_refresh_tokens
			WHERE admin_user_id = $2
			  AND revoked = FALSE
			  AND expires_at > $3
			ORDER BY created_at DESC
			LIMIT 1
		)
	`

	result, err := r.db.Exec(query, time.Now(), adminUserID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to revoke most recent admin token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no active admin tokens found to revoke")
	}

	return nil
}
