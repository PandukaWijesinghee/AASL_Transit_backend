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

// RefreshTokenRepository handles refresh token database operations
type RefreshTokenRepository struct {
	db DB
}

// NewRefreshTokenRepository creates a new refresh token repository
func NewRefreshTokenRepository(db DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{
		db: db,
	}
}

// hashToken creates a SHA-256 hash of the token for storage
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// StoreRefreshToken stores a refresh token in the database
func (r *RefreshTokenRepository) StoreRefreshToken(
	userID uuid.UUID,
	token string,
	deviceID, deviceType, ipAddress, userAgent string,
	expiresAt time.Time,
) error {
	tokenHash := hashToken(token)

	query := `
		INSERT INTO refresh_tokens (
			user_id, token_hash, device_id, device_type,
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
		userID,
		tokenHash,
		deviceIDVal,
		deviceTypeVal,
		ipVal,
		userAgentVal,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}

// GetRefreshToken retrieves a refresh token by its hash
func (r *RefreshTokenRepository) GetRefreshToken(token string) (*models.RefreshToken, error) {
	tokenHash := hashToken(token)

	var refreshToken models.RefreshToken

	query := `
		SELECT id, user_id, token_hash, device_id, device_type,
		       ip_address, user_agent, created_at, expires_at,
		       last_used_at, revoked, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	err := r.db.Get(&refreshToken, query, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Token not found
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return &refreshToken, nil
}

// IsTokenRevoked checks if a refresh token is revoked
func (r *RefreshTokenRepository) IsTokenRevoked(token string) (bool, error) {
	refreshToken, err := r.GetRefreshToken(token)
	if err != nil {
		return false, err
	}

	if refreshToken == nil {
		return true, nil // Token not found, consider it revoked
	}

	return refreshToken.Revoked, nil
}

// IsTokenExpired checks if a refresh token is expired
func (r *RefreshTokenRepository) IsTokenExpired(token string) (bool, error) {
	refreshToken, err := r.GetRefreshToken(token)
	if err != nil {
		return false, err
	}

	if refreshToken == nil {
		return true, nil // Token not found, consider it expired
	}

	return refreshToken.ExpiresAt.Before(time.Now()), nil
}

// RevokeToken revokes a specific refresh token
func (r *RefreshTokenRepository) RevokeToken(token string) error {
	tokenHash := hashToken(token)

	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE token_hash = $2 AND revoked = FALSE
	`

	result, err := r.db.Exec(query, time.Now(), tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("token not found or already revoked")
	}

	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (r *RefreshTokenRepository) RevokeAllUserTokens(userID uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE user_id = $2 AND revoked = FALSE
	`

	_, err := r.db.Exec(query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user tokens: %w", err)
	}

	return nil
}

// RevokeDeviceTokens revokes all tokens for a specific device
func (r *RefreshTokenRepository) RevokeDeviceTokens(userID uuid.UUID, deviceID string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE user_id = $2 AND device_id = $3 AND revoked = FALSE
	`

	_, err := r.db.Exec(query, time.Now(), userID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to revoke device tokens: %w", err)
	}

	return nil
}

// UpdateLastUsed updates the last_used_at timestamp for a token
func (r *RefreshTokenRepository) UpdateLastUsed(token string) error {
	tokenHash := hashToken(token)

	query := `
		UPDATE refresh_tokens
		SET last_used_at = $1
		WHERE token_hash = $2
	`

	_, err := r.db.Exec(query, time.Now(), tokenHash)
	if err != nil {
		return fmt.Errorf("failed to update last used timestamp: %w", err)
	}

	return nil
}

// CleanupExpiredTokens removes expired refresh tokens
func (r *RefreshTokenRepository) CleanupExpiredTokens() (int64, error) {
	query := `
		DELETE FROM refresh_tokens
		WHERE expires_at < $1
	`

	result, err := r.db.Exec(query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// CleanupRevokedTokens removes revoked tokens older than specified duration
func (r *RefreshTokenRepository) CleanupRevokedTokens(olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `
		DELETE FROM refresh_tokens
		WHERE revoked = TRUE AND revoked_at < $1
	`

	result, err := r.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup revoked tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// GetUserTokens retrieves all active tokens for a user
func (r *RefreshTokenRepository) GetUserTokens(userID uuid.UUID) ([]*models.RefreshToken, error) {
	var tokens []*models.RefreshToken

	query := `
		SELECT id, user_id, token_hash, device_id, device_type,
		       ip_address, user_agent, created_at, expires_at,
		       last_used_at, revoked, revoked_at
		FROM refresh_tokens
		WHERE user_id = $1 AND revoked = FALSE AND expires_at > $2
		ORDER BY created_at DESC
	`

	err := r.db.Select(&tokens, query, userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get user tokens: %w", err)
	}

	return tokens, nil
}

// CountUserTokens counts active tokens for a user
func (r *RefreshTokenRepository) CountUserTokens(userID uuid.UUID) (int, error) {
	var count int

	query := `
		SELECT COUNT(*)
		FROM refresh_tokens
		WHERE user_id = $1 AND revoked = FALSE AND expires_at > $2
	`

	err := r.db.QueryRow(query, userID, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count user tokens: %w", err)
	}

	return count, nil
}

// RevokeMostRecentToken revokes the most recent active token for a user
// This is useful when logout is called without a specific refresh token
func (r *RefreshTokenRepository) RevokeMostRecentToken(userID uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE,
		    revoked_at = $1
		WHERE id = (
			SELECT id
			FROM refresh_tokens
			WHERE user_id = $2 
			  AND revoked = FALSE 
			  AND expires_at > $3
			ORDER BY created_at DESC
			LIMIT 1
		)
	`

	result, err := r.db.Exec(query, time.Now(), userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to revoke most recent token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no active tokens found to revoke")
	}

	return nil
}
