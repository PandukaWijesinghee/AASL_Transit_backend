package models

import (
	"time"
)

// SystemSetting represents a system-wide configuration setting
type SystemSetting struct {
	ID           string    `json:"id" db:"id"`
	SettingKey   string    `json:"setting_key" db:"setting_key"`
	SettingValue string    `json:"setting_value" db:"setting_value"`
	Description  *string   `json:"description,omitempty" db:"description"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// UpdateSystemSettingRequest represents the request to update a system setting
type UpdateSystemSettingRequest struct {
	SettingValue string `json:"setting_value" binding:"required"`
}
