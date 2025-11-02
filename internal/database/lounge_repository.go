package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeRepository handles database operations for lounges
type LoungeRepository struct {
	db *sqlx.DB
}

// NewLoungeRepository creates a new lounge repository
func NewLoungeRepository(db *sqlx.DB) *LoungeRepository {
	return &LoungeRepository{db: db}
}

// CreateLounge creates a new lounge (Step 3 of registration)
func (r *LoungeRepository) CreateLounge(
	loungeOwnerID uuid.UUID,
	loungeName string,
	address string,
	city string,
	contactPhone string,
	latitude *string,
	longitude *string,
	price1Hour *string,
	price2Hours *string,
	priceUntilBus *string,
	amenities []byte,
	images []byte,
) (*models.Lounge, error) {
	lounge := &models.Lounge{
		ID:            uuid.New(),
		LoungeOwnerID: loungeOwnerID,
		Status:        models.LoungeStatusPending,
		IsOperational: true,
		TotalStaff:    0,
		TotalBookings: 0,
	}

	query := `
		INSERT INTO lounges (
			id, lounge_owner_id, lounge_name, address, city, 
			contact_phone, latitude, longitude,
			price_1_hour, price_2_hours, price_until_bus,
			amenities, images,
			status, is_operational, total_staff, total_bookings,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 
			$14, $15, $16, $17, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowx(
		query,
		lounge.ID,
		loungeOwnerID,
		loungeName,
		address,
		city,
		contactPhone,
		latitude,
		longitude,
		price1Hour,
		price2Hours,
		priceUntilBus,
		amenities,
		images,
		lounge.Status,
		lounge.IsOperational,
		lounge.TotalStaff,
		lounge.TotalBookings,
	).Scan(&lounge.ID, &lounge.CreatedAt, &lounge.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create lounge: %w", err)
	}

	return lounge, nil
}

// GetLoungeByID retrieves a lounge by ID
func (r *LoungeRepository) GetLoungeByID(id uuid.UUID) (*models.Lounge, error) {
	var lounge models.Lounge
	query := `SELECT * FROM lounges WHERE id = $1`
	err := r.db.Get(&lounge, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lounge: %w", err)
	}
	return &lounge, nil
}

// GetLoungesByOwnerID retrieves all lounges for a specific owner
func (r *LoungeRepository) GetLoungesByOwnerID(ownerID uuid.UUID) ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `
		SELECT * FROM lounges 
		WHERE lounge_owner_id = $1 
		ORDER BY created_at DESC
	`
	err := r.db.Select(&lounges, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges: %w", err)
	}
	return lounges, nil
}

// GetLoungesByCity retrieves all active lounges in a city
func (r *LoungeRepository) GetLoungesByCity(city string) ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `
		SELECT * FROM lounges 
		WHERE city = $1 AND status = 'active' AND is_operational = true
		ORDER BY lounge_name
	`
	err := r.db.Select(&lounges, query, city)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges by city: %w", err)
	}
	return lounges, nil
}

// UpdateLounge updates lounge information
func (r *LoungeRepository) UpdateLounge(
	id uuid.UUID,
	loungeName string,
	address string,
	city string,
	contactPhone string,
	latitude *string,
	longitude *string,
	price1Hour *string,
	price2Hours *string,
	priceUntilBus *string,
	amenities []byte,
	images []byte,
) error {
	query := `
		UPDATE lounges 
		SET 
			lounge_name = $1,
			address = $2,
			city = $3,
			contact_phone = $4,
			latitude = $5,
			longitude = $6,
			price_1_hour = $7,
			price_2_hours = $8,
			price_until_bus = $9,
			amenities = $10,
			images = $11,
			updated_at = NOW()
		WHERE id = $12
	`

	result, err := r.db.Exec(
		query,
		loungeName,
		address,
		city,
		contactPhone,
		latitude,
		longitude,
		price1Hour,
		price2Hours,
		priceUntilBus,
		amenities,
		images,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update lounge: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lounge not found")
	}

	return nil
}

// UpdateLoungeStatus updates lounge status
func (r *LoungeRepository) UpdateLoungeStatus(id uuid.UUID, status string) error {
	query := `
		UPDATE lounges 
		SET 
			status = $1,
			updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update lounge status: %w", err)
	}

	return nil
}

// DeleteLounge deletes a lounge
func (r *LoungeRepository) DeleteLounge(id uuid.UUID) error {
	query := `DELETE FROM lounges WHERE id = $1`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete lounge: %w", err)
	}
	return nil
}

// GetPendingLounges retrieves all lounges pending approval
func (r *LoungeRepository) GetPendingLounges(limit int, offset int) ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `
		SELECT * FROM lounges 
		WHERE status = 'pending'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	err := r.db.Select(&lounges, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending lounges: %w", err)
	}
	return lounges, nil
}
