package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// UserSessionRepository handles user session database operations
type UserSessionRepository struct {
	db DB
}

// NewUserSessionRepository creates a new user session repository
func NewUserSessionRepository(db DB) *UserSessionRepository {
	return &UserSessionRepository{
		db: db,
	}
}

// CreateOrUpdateSession creates a new session or updates existing one for the device
func (r *UserSessionRepository) CreateOrUpdateSession(
	userID uuid.UUID,
	deviceID, deviceType, deviceModel, appVersion, osVersion string,
	ipAddress, fcmToken string,
) (*models.UserSession, error) {
	// Check if session exists for this user and device
	existingSession, err := r.GetByUserAndDevice(userID, deviceID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing session: %w", err)
	}

	if existingSession != nil {
		// Update existing session
		return r.UpdateSession(existingSession.ID, deviceModel, appVersion, osVersion, ipAddress, fcmToken)
	}

	// Create new session
	query := `
		INSERT INTO user_sessions (
			id, user_id, device_id, device_type, device_model, app_version, os_version,
			fcm_token, ip_address, last_activity_at, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		RETURNING id, user_id, device_id, device_type, device_model, app_version, os_version,
			fcm_token, ip_address, location_permission, notification_permission,
			last_activity_at, is_active, created_at, updated_at
	`

	session := &models.UserSession{}
	now := time.Now()
	sessionID := uuid.New()

	err = r.db.QueryRow(
		query,
		sessionID,
		userID,
		deviceID,
		deviceType,
		nullString(deviceModel),
		nullString(appVersion),
		nullString(osVersion),
		nullString(fcmToken),
		nullString(ipAddress),
		now,    // last_activity_at
		true,   // is_active
		now,    // created_at
		now,    // updated_at
	).Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceID,
		&session.DeviceType,
		&session.DeviceModel,
		&session.AppVersion,
		&session.OSVersion,
		&session.FCMToken,
		&session.IPAddress,
		&session.LocationPermission,
		&session.NotificationPermission,
		&session.LastActivityAt,
		&session.IsActive,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GetByUserAndDevice retrieves a session by user ID and device ID
func (r *UserSessionRepository) GetByUserAndDevice(userID uuid.UUID, deviceID string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, device_id, device_type, device_model, app_version, os_version,
			fcm_token, ip_address, location_permission, notification_permission,
			last_activity_at, is_active, created_at, updated_at
		FROM user_sessions
		WHERE user_id = $1 AND device_id = $2
		LIMIT 1
	`

	session := &models.UserSession{}
	err := r.db.QueryRow(query, userID, deviceID).Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceID,
		&session.DeviceType,
		&session.DeviceModel,
		&session.AppVersion,
		&session.OSVersion,
		&session.FCMToken,
		&session.IPAddress,
		&session.LocationPermission,
		&session.NotificationPermission,
		&session.LastActivityAt,
		&session.IsActive,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// UpdateSession updates an existing session
func (r *UserSessionRepository) UpdateSession(
	sessionID uuid.UUID,
	deviceModel, appVersion, osVersion, ipAddress, fcmToken string,
) (*models.UserSession, error) {
	query := `
		UPDATE user_sessions
		SET device_model = $2,
		    app_version = $3,
		    os_version = $4,
		    ip_address = $5,
		    fcm_token = $6,
		    last_activity_at = $7,
		    updated_at = $8,
		    is_active = true
		WHERE id = $1
		RETURNING id, user_id, device_id, device_type, device_model, app_version, os_version,
			fcm_token, ip_address, location_permission, notification_permission,
			last_activity_at, is_active, created_at, updated_at
	`

	session := &models.UserSession{}
	now := time.Now()

	err := r.db.QueryRow(
		query,
		sessionID,
		nullString(deviceModel),
		nullString(appVersion),
		nullString(osVersion),
		nullString(ipAddress),
		nullString(fcmToken),
		now, // last_activity_at
		now, // updated_at
	).Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceID,
		&session.DeviceType,
		&session.DeviceModel,
		&session.AppVersion,
		&session.OSVersion,
		&session.FCMToken,
		&session.IPAddress,
		&session.LocationPermission,
		&session.NotificationPermission,
		&session.LastActivityAt,
		&session.IsActive,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return session, nil
}

// UpdateLastActivity updates the last activity timestamp for a session
func (r *UserSessionRepository) UpdateLastActivity(userID uuid.UUID, deviceID string) error {
	query := `
		UPDATE user_sessions
		SET last_activity_at = $1,
		    updated_at = $1
		WHERE user_id = $2 AND device_id = $3
	`

	_, err := r.db.Exec(query, time.Now(), userID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to update last activity: %w", err)
	}

	return nil
}

// UpdatePermissions updates location and notification permissions
func (r *UserSessionRepository) UpdatePermissions(
	userID uuid.UUID,
	deviceID string,
	locationPermission, notificationPermission bool,
) error {
	query := `
		UPDATE user_sessions
		SET location_permission = $1,
		    notification_permission = $2,
		    updated_at = $3
		WHERE user_id = $4 AND device_id = $5
	`

	_, err := r.db.Exec(
		query,
		locationPermission,
		notificationPermission,
		time.Now(),
		userID,
		deviceID,
	)

	if err != nil {
		return fmt.Errorf("failed to update permissions: %w", err)
	}

	return nil
}

// UpdateFCMToken updates the FCM token for push notifications
func (r *UserSessionRepository) UpdateFCMToken(userID uuid.UUID, deviceID, fcmToken string) error {
	query := `
		UPDATE user_sessions
		SET fcm_token = $1,
		    updated_at = $2
		WHERE user_id = $3 AND device_id = $4
	`

	_, err := r.db.Exec(query, fcmToken, time.Now(), userID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to update FCM token: %w", err)
	}

	return nil
}

// DeactivateSession marks a session as inactive (logout)
func (r *UserSessionRepository) DeactivateSession(userID uuid.UUID, deviceID string) error {
	query := `
		UPDATE user_sessions
		SET is_active = false,
		    updated_at = $1
		WHERE user_id = $2 AND device_id = $3
	`

	_, err := r.db.Exec(query, time.Now(), userID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to deactivate session: %w", err)
	}

	return nil
}

// DeactivateAllUserSessions marks all user sessions as inactive (logout from all devices)
func (r *UserSessionRepository) DeactivateAllUserSessions(userID uuid.UUID) error {
	query := `
		UPDATE user_sessions
		SET is_active = false,
		    updated_at = $1
		WHERE user_id = $2
	`

	_, err := r.db.Exec(query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate all user sessions: %w", err)
	}

	return nil
}

// GetActiveSessions retrieves all active sessions for a user
func (r *UserSessionRepository) GetActiveSessions(userID uuid.UUID) ([]*models.UserSession, error) {
	query := `
		SELECT id, user_id, device_id, device_type, device_model, app_version, os_version,
			fcm_token, ip_address, location_permission, notification_permission,
			last_activity_at, is_active, created_at, updated_at
		FROM user_sessions
		WHERE user_id = $1 AND is_active = true
		ORDER BY last_activity_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	defer rows.Close()

	sessions := []*models.UserSession{}
	for rows.Next() {
		session := &models.UserSession{}
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.DeviceID,
			&session.DeviceType,
			&session.DeviceModel,
			&session.AppVersion,
			&session.OSVersion,
			&session.FCMToken,
			&session.IPAddress,
			&session.LocationPermission,
			&session.NotificationPermission,
			&session.LastActivityAt,
			&session.IsActive,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// CleanupInactiveSessions removes inactive sessions older than specified duration
func (r *UserSessionRepository) CleanupInactiveSessions(olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `
		DELETE FROM user_sessions
		WHERE is_active = false AND updated_at < $1
	`

	result, err := r.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup inactive sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// nullString returns sql.NullString for empty strings
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
