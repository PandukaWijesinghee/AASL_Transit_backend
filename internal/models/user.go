package models

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// NullString wraps sql.NullString to provide proper JSON marshaling
type NullString struct {
	sql.NullString
}

// MarshalJSON implements json.Marshaler
func (ns NullString) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(ns.String)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON implements json.Unmarshaler
func (ns *NullString) UnmarshalJSON(data []byte) error {
	var s *string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s != nil {
		ns.Valid = true
		ns.String = *s
	} else {
		ns.Valid = false
	}
	return nil
}

// NullTime wraps sql.NullTime to provide proper JSON marshaling
type NullTime struct {
	sql.NullTime
}

// MarshalJSON implements json.Marshaler
func (nt NullTime) MarshalJSON() ([]byte, error) {
	if nt.Valid {
		return json.Marshal(nt.Time)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON implements json.Unmarshaler
func (nt *NullTime) UnmarshalJSON(data []byte) error {
	var t *time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	if t != nil {
		nt.Valid = true
		nt.Time = *t
	} else {
		nt.Valid = false
	}
	return nil
}

// User represents a user in the system
type User struct {
	ID               uuid.UUID    `json:"id" db:"id"`
	Phone            string       `json:"phone" db:"phone"`
	Email            NullString   `json:"email,omitempty" db:"email"`
	FirstName        NullString   `json:"first_name,omitempty" db:"first_name"`
	LastName         NullString   `json:"last_name,omitempty" db:"last_name"`
	NIC              NullString   `json:"nic,omitempty" db:"nic"`
	DateOfBirth      NullTime     `json:"date_of_birth,omitempty" db:"date_of_birth"`
	Address          NullString   `json:"address,omitempty" db:"address"`
	City             NullString   `json:"city,omitempty" db:"city"`
	PostalCode       NullString   `json:"postal_code,omitempty" db:"postal_code"`
	Roles            pq.StringArray `json:"roles" db:"roles"`
	ProfilePhotoURL  NullString   `json:"profile_photo_url,omitempty" db:"profile_photo_url"`
	ProfileCompleted bool         `json:"profile_completed" db:"profile_completed"`
	Status           string       `json:"status" db:"status"`
	PhoneVerified    bool         `json:"phone_verified" db:"phone_verified"`
	EmailVerified    bool         `json:"email_verified" db:"email_verified"`
	LastLoginAt      NullTime     `json:"last_login_at,omitempty" db:"last_login_at"`
	Metadata         NullString   `json:"metadata,omitempty" db:"metadata"`
	CreatedAt        time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at" db:"updated_at"`
}

// OTPVerification represents an OTP verification record
type OTPVerification struct {
	ID          int64      `json:"id" db:"id"`
	Phone       string     `json:"phone" db:"phone"`
	OTPCode     string     `json:"-" db:"otp_code"` // Never expose in JSON
	Purpose     string     `json:"purpose" db:"purpose"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at" db:"expires_at"`
	Verified    bool       `json:"verified" db:"verified"`
	VerifiedAt  NullTime   `json:"verified_at,omitempty" db:"verified_at"`
	Attempts    int        `json:"attempts" db:"attempts"`
	MaxAttempts int        `json:"max_attempts" db:"max_attempts"`
	IPAddress   NullString `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent   NullString `json:"user_agent,omitempty" db:"user_agent"`
}

// OTPRateLimit represents rate limiting for OTP requests
type OTPRateLimit struct {
	ID            int64     `json:"id" db:"id"`
	Phone         string    `json:"phone" db:"phone"`
	RequestCount  int       `json:"request_count" db:"request_count"`
	WindowStart   time.Time `json:"window_start" db:"window_start"`
	BlockedUntil  NullTime  `json:"blocked_until,omitempty" db:"blocked_until"`
	LastRequestAt time.Time `json:"last_request_at" db:"last_request_at"`
}

// RefreshToken represents a JWT refresh token
type RefreshToken struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash  string     `json:"-" db:"token_hash"` // Never expose
	DeviceID   NullString `json:"device_id,omitempty" db:"device_id"`
	DeviceType NullString `json:"device_type,omitempty" db:"device_type"`
	IPAddress  NullString `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  NullString `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	LastUsedAt NullTime   `json:"last_used_at,omitempty" db:"last_used_at"`
	Revoked    bool       `json:"revoked" db:"revoked"`
	RevokedAt  NullTime   `json:"revoked_at,omitempty" db:"revoked_at"`
}

// UserSession represents an active user session
type UserSession struct {
	ID                     uuid.UUID  `json:"id" db:"id"`
	UserID                 uuid.UUID  `json:"user_id" db:"user_id"`
	DeviceID               string     `json:"device_id" db:"device_id"`
	DeviceType             string     `json:"device_type" db:"device_type"`
	DeviceModel            NullString `json:"device_model,omitempty" db:"device_model"`
	AppVersion             NullString `json:"app_version,omitempty" db:"app_version"`
	OSVersion              NullString `json:"os_version,omitempty" db:"os_version"`
	FCMToken               NullString `json:"fcm_token,omitempty" db:"fcm_token"`
	IPAddress              NullString `json:"ip_address,omitempty" db:"ip_address"`
	LocationPermission     bool       `json:"location_permission" db:"location_permission"`
	NotificationPermission bool       `json:"notification_permission" db:"notification_permission"`
	LastActivityAt         time.Time  `json:"last_activity_at" db:"last_activity_at"`
	IsActive               bool       `json:"is_active" db:"is_active"`
	CreatedAt              time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at" db:"updated_at"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         int64      `json:"id" db:"id"`
	UserID     uuid.NullUUID  `json:"user_id,omitempty" db:"user_id"`
	Action     string     `json:"action" db:"action"`
	EntityType NullString `json:"entity_type,omitempty" db:"entity_type"`
	EntityID   uuid.NullUUID  `json:"entity_id,omitempty" db:"entity_id"`
	IPAddress  NullString `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  NullString `json:"user_agent,omitempty" db:"user_agent"`
	Details    NullString `json:"details,omitempty" db:"details"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// Helper type for nullable UUID
type NullUUID struct {
	uuid.UUID
	Valid bool
}

func (nu *NullUUID) Scan(value interface{}) error {
	if value == nil {
		nu.UUID, nu.Valid = uuid.Nil, false
		return nil
	}
	nu.Valid = true
	return nu.UUID.Scan(value)
}
