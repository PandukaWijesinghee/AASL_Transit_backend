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
	contactPhone string,
	latitude *string,
	longitude *string,
	capacity *int,
	price1Hour *string,
	price2Hours *string,
	price3Hours *string,
	priceUntilBus *string,
	amenities string,
	images string,
) (*models.Lounge, error) {
	lounge := &models.Lounge{
		ID:            uuid.New(),
		LoungeOwnerID: loungeOwnerID,
		Status:        models.LoungeStatusPending,
		IsOperational: true,
	}

	query := `
		INSERT INTO lounges (
			id, lounge_owner_id, lounge_name, address,
			contact_phone, latitude, longitude, capacity,
			price_1_hour, price_2_hours, price_3_hours, price_until_bus,
			amenities, images,
			status, is_operational,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowx(
		query,
		lounge.ID,
		loungeOwnerID,
		loungeName,
		address,
		contactPhone,
		latitude,
		longitude,
		capacity,
		price1Hour,
		price2Hours,
		price3Hours,
		priceUntilBus,
		amenities,
		images,
		lounge.Status,
		lounge.IsOperational,
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

// GetAllActiveLounges retrieves all active lounges (for public listing)
func (r *LoungeRepository) GetAllActiveLounges() ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `
		SELECT * FROM lounges 
		WHERE status = 'approved' AND is_operational = true
		ORDER BY lounge_name
	`
	err := r.db.Select(&lounges, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active lounges: %w", err)
	}
	return lounges, nil
}

// GetLoungesByStatus retrieves all lounges with a specific status
func (r *LoungeRepository) GetLoungesByStatus(status string) ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `SELECT * FROM lounges WHERE status = $1 ORDER BY created_at DESC`
	err := r.db.Select(&lounges, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges by status: %w", err)
	}
	return lounges, nil
}

// SearchActiveLounges retrieves active lounges with optional state filter and limit
func (r *LoungeRepository) SearchActiveLounges(state string, limit int) ([]models.Lounge, error) {
	var lounges []models.Lounge
	var args []interface{}
	argNum := 1

	query := `SELECT * FROM lounges WHERE status = 'approved' AND is_operational = true`

	if state != "" {
		query += fmt.Sprintf(" AND LOWER(state) = LOWER($%d)", argNum)
		args = append(args, state)
		argNum++
	}

	// Add random ordering when limit is specified, otherwise order by name
	if limit > 0 {
		query += " ORDER BY RANDOM()"
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, limit)
	} else {
		query += " ORDER BY lounge_name"
	}

	err := r.db.Select(&lounges, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search active lounges: %w", err)
	}
	return lounges, nil
}

// GetLoungesByStopID retrieves all active lounges that serve a specific stop
// A lounge serves a stop if the stop is either stop_before_id or stop_after_id in lounge_routes
func (r *LoungeRepository) GetLoungesByStopID(stopID uuid.UUID) ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `
		SELECT DISTINCT l.* 
		FROM lounges l
		INNER JOIN lounge_routes lr ON l.id = lr.lounge_id
		WHERE l.status = 'approved' 
		  AND l.is_operational = true
		  AND (lr.stop_before_id = $1 OR lr.stop_after_id = $1)
		ORDER BY l.lounge_name
	`
	err := r.db.Select(&lounges, query, stopID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges by stop: %w", err)
	}
	return lounges, nil
}

// GetLoungesByRouteID retrieves all active lounges that serve a specific route
func (r *LoungeRepository) GetLoungesByRouteID(routeID uuid.UUID) ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `
		SELECT DISTINCT l.* 
		FROM lounges l
		INNER JOIN lounge_routes lr ON l.id = lr.lounge_id
		WHERE l.status = 'approved' 
		  AND l.is_operational = true
		  AND lr.master_route_id = $1
		ORDER BY l.lounge_name
	`
	err := r.db.Select(&lounges, query, routeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges by route: %w", err)
	}
	return lounges, nil
}

// GetLoungesNearStop retrieves lounges where passenger's stop is within N stops of the lounge's location
// The lounge is located between stop_before_id and stop_after_id on the route
// We check if the passenger's stop_order is within 'maxStopDistance' of either lounge stop
func (r *LoungeRepository) GetLoungesNearStop(masterRouteID uuid.UUID, passengerStopID uuid.UUID, maxStopDistance int) ([]models.Lounge, error) {
	var lounges []models.Lounge
	query := `
		WITH passenger_stop AS (
			-- Get the passenger's selected stop order
			SELECT stop_order 
			FROM master_route_stops 
			WHERE id = $2 AND master_route_id = $1
		),
		lounge_stops AS (
			-- Get lounges on this route with their stop orders
			SELECT 
				lr.lounge_id,
				mrs_before.stop_order as stop_before_order,
				mrs_after.stop_order as stop_after_order
			FROM lounge_routes lr
			LEFT JOIN master_route_stops mrs_before ON lr.stop_before_id = mrs_before.id
			LEFT JOIN master_route_stops mrs_after ON lr.stop_after_id = mrs_after.id
			WHERE lr.master_route_id = $1
		)
		SELECT DISTINCT l.* 
		FROM lounges l
		INNER JOIN lounge_stops ls ON l.id = ls.lounge_id
		CROSS JOIN passenger_stop ps
		WHERE l.status = 'approved' 
		  AND l.is_operational = true
		  AND (
		      -- Passenger stop is within N stops of stop_before
		      ABS(ps.stop_order - COALESCE(ls.stop_before_order, 0)) <= $3
		      OR
		      -- Passenger stop is within N stops of stop_after  
		      ABS(ps.stop_order - COALESCE(ls.stop_after_order, 0)) <= $3
		  )
		ORDER BY l.lounge_name
	`
	err := r.db.Select(&lounges, query, masterRouteID, passengerStopID, maxStopDistance)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounges near stop: %w", err)
	}
	return lounges, nil
}

// GetDistinctStates retrieves all distinct states from active lounges
func (r *LoungeRepository) GetDistinctStates() ([]string, error) {
	var states []string
	query := `
		SELECT DISTINCT state FROM lounges 
		WHERE status = 'approved' AND is_operational = true AND state IS NOT NULL AND state != ''
		ORDER BY state
	`
	err := r.db.Select(&states, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct states: %w", err)
	}
	return states, nil
}

// UpdateLounge updates lounge information
func (r *LoungeRepository) UpdateLounge(
	id uuid.UUID,
	loungeName string,
	address string,
	contactPhone string,
	latitude *string,
	longitude *string,
	capacity *int,
	price1Hour *string,
	price2Hours *string,
	price3Hours *string,
	priceUntilBus *string,
	amenities string,
	images string,
) error {
	query := `
		UPDATE lounges 
		SET 
			lounge_name = $1,
			address = $2,
			contact_phone = $3,
			latitude = $4,
			longitude = $5,
			capacity = $6,
			price_1_hour = $7,
			price_2_hours = $8,
			price_3_hours = $9,
			price_until_bus = $10,
			amenities = $11,
			images = $12,
			updated_at = NOW()
		WHERE id = $13
	`

	result, err := r.db.Exec(
		query,
		loungeName,
		address,
		contactPhone,
		latitude,
		longitude,
		capacity,
		price1Hour,
		price2Hours,
		price3Hours,
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
