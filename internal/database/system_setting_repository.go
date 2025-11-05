package database

import (
	"database/sql"
	"strconv"

	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// SystemSettingRepository handles database operations for system_settings table
type SystemSettingRepository struct {
	db DB
}

// NewSystemSettingRepository creates a new SystemSettingRepository
func NewSystemSettingRepository(db DB) *SystemSettingRepository {
	return &SystemSettingRepository{db: db}
}

// GetAll retrieves all system settings
func (r *SystemSettingRepository) GetAll() ([]models.SystemSetting, error) {
	query := `
		SELECT id, setting_key, setting_value, description, created_at, updated_at
		FROM system_settings
		ORDER BY setting_key
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := []models.SystemSetting{}
	for rows.Next() {
		var setting models.SystemSetting
		var description sql.NullString

		err := rows.Scan(
			&setting.ID,
			&setting.SettingKey,
			&setting.SettingValue,
			&description,
			&setting.CreatedAt,
			&setting.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if description.Valid {
			setting.Description = &description.String
		}

		settings = append(settings, setting)
	}

	return settings, rows.Err()
}

// GetByKey retrieves a system setting by its key
func (r *SystemSettingRepository) GetByKey(key string) (*models.SystemSetting, error) {
	query := `
		SELECT id, setting_key, setting_value, description, created_at, updated_at
		FROM system_settings
		WHERE setting_key = $1
	`

	var setting models.SystemSetting
	var description sql.NullString

	err := r.db.QueryRow(query, key).Scan(
		&setting.ID,
		&setting.SettingKey,
		&setting.SettingValue,
		&description,
		&setting.CreatedAt,
		&setting.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if description.Valid {
		setting.Description = &description.String
	}

	return &setting, nil
}

// Update updates a system setting's value
func (r *SystemSettingRepository) Update(key string, value string) error {
	query := `
		UPDATE system_settings
		SET setting_value = $1, updated_at = NOW()
		WHERE setting_key = $2
	`

	result, err := r.db.Exec(query, value, key)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetIntValue retrieves a system setting as an integer
func (r *SystemSettingRepository) GetIntValue(key string, defaultValue int) int {
	setting, err := r.GetByKey(key)
	if err != nil {
		return defaultValue
	}

	value, err := strconv.Atoi(setting.SettingValue)
	if err != nil {
		return defaultValue
	}

	return value
}
