package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/utils"
)

// AuditService handles audit logging for security events
type AuditService struct {
	db database.DB
}

// NewAuditService creates a new audit service
func NewAuditService(db database.DB) *AuditService {
	return &AuditService{
		db: db,
	}
}

// AuditEvent represents a security event to be logged
type AuditEvent struct {
	UserID      *uuid.UUID             // Can be nil for pre-authentication events
	Action      string                 // Action type (e.g., "otp_request", "otp_verify", "login", "logout")
	EntityType  string                 // Type of entity affected (e.g., "otp", "user", "session")
	EntityID    *uuid.UUID             // ID of the affected entity (can be nil)
	IPAddress   string                 // Client IP address
	UserAgent   string                 // Client user agent
	Details     map[string]interface{} // Additional details as JSONB
}

// LogOTPRequest logs an OTP generation request
func (s *AuditService) LogOTPRequest(phone, ipAddress, userAgent string, success bool, reason string) error {
	deviceInfo := utils.ParseUserAgent(userAgent)

	details := map[string]interface{}{
		"phone":       phone,
		"success":     success,
		"device_info": deviceInfo,
	}

	if reason != "" {
		details["reason"] = reason
	}

	return s.logEvent(AuditEvent{
		UserID:     nil, // No user ID yet (pre-authentication)
		Action:     "otp_request",
		EntityType: "otp",
		EntityID:   nil,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
	})
}

// LogOTPVerification logs an OTP verification attempt
func (s *AuditService) LogOTPVerification(userID *uuid.UUID, phone string, success bool, attempts int, ipAddress, userAgent, failureReason string) error {
	deviceInfo := utils.ParseUserAgent(userAgent)

	details := map[string]interface{}{
		"phone":       phone,
		"success":     success,
		"attempts":    attempts,
		"device_info": deviceInfo,
	}

	if !success && failureReason != "" {
		details["failure_reason"] = failureReason
	}

	action := "otp_verify_failed"
	if success {
		action = "otp_verify_success"
	}

	return s.logEvent(AuditEvent{
		UserID:     userID,
		Action:     action,
		EntityType: "otp",
		EntityID:   nil,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
	})
}

// LogRateLimitViolation logs a rate limit violation event
func (s *AuditService) LogRateLimitViolation(phone, ipAddress, userAgent, limitType string, retryAfter time.Time) error {
	deviceInfo := utils.ParseUserAgent(userAgent)

	details := map[string]interface{}{
		"phone":       phone,
		"limit_type":  limitType, // "phone" or "ip"
		"retry_after": retryAfter,
		"device_info": deviceInfo,
	}

	return s.logEvent(AuditEvent{
		UserID:     nil,
		Action:     "rate_limit_violation",
		EntityType: "rate_limit",
		EntityID:   nil,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
	})
}

// LogLogin logs a successful login event
func (s *AuditService) LogLogin(userID uuid.UUID, phone, ipAddress, userAgent, deviceID, deviceType string) error {
	deviceInfo := utils.ParseUserAgent(userAgent)

	details := map[string]interface{}{
		"phone":       phone,
		"device_id":   deviceID,
		"device_type": deviceType,
		"device_info": deviceInfo,
	}

	return s.logEvent(AuditEvent{
		UserID:     &userID,
		Action:     "login",
		EntityType: "user",
		EntityID:   &userID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
	})
}

// LogLogout logs a logout event
func (s *AuditService) LogLogout(userID uuid.UUID, ipAddress, userAgent string, logoutAll bool) error {
	deviceInfo := utils.ParseUserAgent(userAgent)

	details := map[string]interface{}{
		"logout_all":  logoutAll,
		"device_info": deviceInfo,
	}

	return s.logEvent(AuditEvent{
		UserID:     &userID,
		Action:     "logout",
		EntityType: "user",
		EntityID:   &userID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
	})
}

// LogTokenRefresh logs a refresh token usage event
func (s *AuditService) LogTokenRefresh(userID uuid.UUID, ipAddress, userAgent string, success bool) error {
	deviceInfo := utils.ParseUserAgent(userAgent)

	action := "token_refresh_success"
	if !success {
		action = "token_refresh_failed"
	}

	details := map[string]interface{}{
		"success":     success,
		"device_info": deviceInfo,
	}

	return s.logEvent(AuditEvent{
		UserID:     &userID,
		Action:     action,
		EntityType: "token",
		EntityID:   nil,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
	})
}

// LogSuspiciousActivity logs suspicious security events
func (s *AuditService) LogSuspiciousActivity(userID *uuid.UUID, activity, ipAddress, userAgent string, details map[string]interface{}) error {
	if details == nil {
		details = make(map[string]interface{})
	}

	deviceInfo := utils.ParseUserAgent(userAgent)
	details["device_info"] = deviceInfo
	details["activity"] = activity

	return s.logEvent(AuditEvent{
		UserID:     userID,
		Action:     "suspicious_activity",
		EntityType: "security",
		EntityID:   nil,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
	})
}

// logEvent is the internal method that writes to the audit_logs table
func (s *AuditService) logEvent(event AuditEvent) error {
	query := `
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, ip_address, user_agent, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`

	// Convert details map to JSONB (Postgres will handle the conversion)
	_, err := s.db.Exec(
		query,
		event.UserID,
		event.Action,
		event.EntityType,
		event.EntityID,
		event.IPAddress,
		event.UserAgent,
		event.Details,
	)

	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

// GetRecentEvents retrieves recent audit events for a user
func (s *AuditService) GetRecentEvents(userID uuid.UUID, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT action, entity_type, ip_address, user_agent, details, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent events: %w", err)
	}
	defer rows.Close()

	events := []map[string]interface{}{}
	for rows.Next() {
		var action, entityType, ipAddress, userAgent string
		var details map[string]interface{}
		var createdAt time.Time

		err := rows.Scan(&action, &entityType, &ipAddress, &userAgent, &details, &createdAt)
		if err != nil {
			continue
		}

		events = append(events, map[string]interface{}{
			"action":      action,
			"entity_type": entityType,
			"ip_address":  ipAddress,
			"user_agent":  userAgent,
			"details":     details,
			"created_at":  createdAt,
		})
	}

	return events, nil
}

// CleanupOldAuditLogs removes audit logs older than the specified duration
func (s *AuditService) CleanupOldAuditLogs(olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `
		DELETE FROM audit_logs
		WHERE created_at < $1
	`

	result, err := s.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old audit logs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}
