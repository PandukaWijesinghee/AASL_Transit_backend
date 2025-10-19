package services

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/smarttransit/sms-auth-backend/internal/database"
)

// RateLimitService handles OTP request rate limiting
type RateLimitService struct {
	db database.DB
}

// NewRateLimitService creates a new rate limit service
func NewRateLimitService(db database.DB) *RateLimitService {
	return &RateLimitService{
		db: db,
	}
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	MaxPhoneRequests int           // Max OTP requests per phone
	PhoneWindow      time.Duration // Time window for phone rate limit
	MaxIPRequests    int           // Max OTP requests per IP
	IPWindow         time.Duration // Time window for IP rate limit
}

// DefaultRateLimitConfig returns the default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxPhoneRequests: 3,                // 3 requests
		PhoneWindow:      10 * time.Minute, // per 10 minutes
		MaxIPRequests:    10,               // 10 requests
		IPWindow:         1 * time.Hour,    // per hour
	}
}

// RateLimitError represents a rate limit exceeded error
type RateLimitError struct {
	Message    string
	RetryAfter time.Time
	Type       string // "phone" or "ip"
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// CheckOTPRateLimit checks if a phone number or IP has exceeded rate limits
func (s *RateLimitService) CheckOTPRateLimit(phone, ip string) error {
	config := DefaultRateLimitConfig()

	// Check phone-based rate limit
	if phone != "" {
		phoneCount, lastRequest, err := s.getRequestCount(phone, "phone", config.PhoneWindow)
		if err != nil {
			return fmt.Errorf("failed to check phone rate limit: %w", err)
		}

		if phoneCount >= config.MaxPhoneRequests {
			retryAfter := lastRequest.Add(config.PhoneWindow)
			return &RateLimitError{
				Message:    fmt.Sprintf("Too many OTP requests for this phone number. Please try again after %s", retryAfter.Format("15:04:05")),
				RetryAfter: retryAfter,
				Type:       "phone",
			}
		}
	}

	// Check IP-based rate limit
	if ip != "" {
		ipCount, lastRequest, err := s.getRequestCount(ip, "ip", config.IPWindow)
		if err != nil {
			return fmt.Errorf("failed to check IP rate limit: %w", err)
		}

		if ipCount >= config.MaxIPRequests {
			retryAfter := lastRequest.Add(config.IPWindow)
			return &RateLimitError{
				Message:    fmt.Sprintf("Too many OTP requests from this IP address. Please try again after %s", retryAfter.Format("15:04:05")),
				RetryAfter: retryAfter,
				Type:       "ip",
			}
		}
	}

	return nil
}

// getRequestCount gets the number of requests within the time window
func (s *RateLimitService) getRequestCount(identifier, identifierType string, window time.Duration) (int, time.Time, error) {
	windowStart := time.Now().Add(-window)

	query := `
		SELECT COUNT(*), COALESCE(MAX(created_at), NOW())
		FROM otp_rate_limits
		WHERE identifier = $1 
		  AND identifier_type = $2 
		  AND created_at > $3
	`

	var count int
	var lastRequest time.Time

	err := s.db.QueryRow(query, identifier, identifierType, windowStart).Scan(&count, &lastRequest)
	if err != nil && err != sql.ErrNoRows {
		return 0, time.Time{}, err
	}

	return count, lastRequest, nil
}

// RecordOTPRequest records an OTP request for rate limiting
func (s *RateLimitService) RecordOTPRequest(phone, ip string) error {
	// Record phone-based request
	if phone != "" {
		err := s.recordRequest(phone, "phone")
		if err != nil {
			return fmt.Errorf("failed to record phone request: %w", err)
		}
	}

	// Record IP-based request
	if ip != "" {
		err := s.recordRequest(ip, "ip")
		if err != nil {
			return fmt.Errorf("failed to record IP request: %w", err)
		}
	}

	return nil
}

// recordRequest inserts a rate limit record
func (s *RateLimitService) recordRequest(identifier, identifierType string) error {
	query := `
		INSERT INTO otp_rate_limits (identifier, identifier_type, created_at)
		VALUES ($1, $2, NOW())
	`

	_, err := s.db.Exec(query, identifier, identifierType)
	return err
}

// CleanupExpiredRateLimits removes old rate limit records
func (s *RateLimitService) CleanupExpiredRateLimits() (int64, error) {
	config := DefaultRateLimitConfig()

	// Delete records older than the longest window (IP window is 1 hour)
	maxWindow := config.IPWindow
	if config.PhoneWindow > maxWindow {
		maxWindow = config.PhoneWindow
	}

	cutoffTime := time.Now().Add(-maxWindow)

	query := `
		DELETE FROM otp_rate_limits
		WHERE created_at < $1
	`

	result, err := s.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup rate limits: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// GetRateLimitStatus returns the current rate limit status for a phone or IP
func (s *RateLimitService) GetRateLimitStatus(identifier, identifierType string) (int, time.Time, error) {
	config := DefaultRateLimitConfig()

	window := config.PhoneWindow
	if identifierType == "ip" {
		window = config.IPWindow
	}

	count, lastRequest, err := s.getRequestCount(identifier, identifierType, window)
	if err != nil {
		return 0, time.Time{}, err
	}

	return count, lastRequest, nil
}

// IsRateLimited checks if an identifier is currently rate limited
func (s *RateLimitService) IsRateLimited(identifier, identifierType string) (bool, time.Time, error) {
	config := DefaultRateLimitConfig()

	window := config.PhoneWindow
	maxRequests := config.MaxPhoneRequests
	if identifierType == "ip" {
		window = config.IPWindow
		maxRequests = config.MaxIPRequests
	}

	count, lastRequest, err := s.getRequestCount(identifier, identifierType, window)
	if err != nil {
		return false, time.Time{}, err
	}

	if count >= maxRequests {
		retryAfter := lastRequest.Add(window)
		return true, retryAfter, nil
	}

	return false, time.Time{}, nil
}
