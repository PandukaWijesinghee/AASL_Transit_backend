package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeRouteRepository handles database operations for lounge routes
type LoungeRouteRepository struct {
	db *sqlx.DB
}

// NewLoungeRouteRepository creates a new lounge route repository
func NewLoungeRouteRepository(db *sqlx.DB) *LoungeRouteRepository {
	return &LoungeRouteRepository{db: db}
}

// CreateLoungeRoute adds a new route to a lounge
func (r *LoungeRouteRepository) CreateLoungeRoute(loungeRoute *models.LoungeRoute) error {
	query := `
		INSERT INTO lounge_routes (
			id, lounge_id, master_route_id, stop_before_id, stop_after_id
		) VALUES (
			$1, $2, $3, $4, $5
		)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(
		query,
		loungeRoute.ID,
		loungeRoute.LoungeID,
		loungeRoute.MasterRouteID,
		loungeRoute.StopBeforeID,
		loungeRoute.StopAfterID,
	).Scan(&loungeRoute.CreatedAt, &loungeRoute.UpdatedAt)
}

// GetLoungeRoutes retrieves all routes for a specific lounge
func (r *LoungeRouteRepository) GetLoungeRoutes(loungeID uuid.UUID) ([]models.LoungeRoute, error) {
	var routes []models.LoungeRoute

	query := `
		SELECT 
			id, lounge_id, master_route_id, stop_before_id, stop_after_id,
			created_at, updated_at
		FROM lounge_routes
		WHERE lounge_id = $1
		ORDER BY created_at DESC
	`

	err := r.db.Select(&routes, query, loungeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lounge routes: %w", err)
	}

	return routes, nil
}

// GetLoungeRoute retrieves a specific lounge route by ID
func (r *LoungeRouteRepository) GetLoungeRoute(id uuid.UUID) (*models.LoungeRoute, error) {
	var loungeRoute models.LoungeRoute

	query := `
		SELECT 
			id, lounge_id, master_route_id, stop_before_id, stop_after_id,
			created_at, updated_at
		FROM lounge_routes
		WHERE id = $1
	`

	err := r.db.Get(&loungeRoute, query, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("lounge route not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lounge route: %w", err)
	}

	return &loungeRoute, nil
}

// UpdateLoungeRoute updates an existing lounge route
func (r *LoungeRouteRepository) UpdateLoungeRoute(loungeRoute *models.LoungeRoute) error {
	query := `
		UPDATE lounge_routes
		SET 
			master_route_id = $1,
			stop_before_id = $2,
			stop_after_id = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING updated_at
	`

	return r.db.QueryRow(
		query,
		loungeRoute.MasterRouteID,
		loungeRoute.StopBeforeID,
		loungeRoute.StopAfterID,
		loungeRoute.ID,
	).Scan(&loungeRoute.UpdatedAt)
}

// DeleteLoungeRoute removes a route from a lounge
func (r *LoungeRouteRepository) DeleteLoungeRoute(id uuid.UUID) error {
	query := `DELETE FROM lounge_routes WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete lounge route: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lounge route not found")
	}

	return nil
}

// DeleteAllLoungeRoutes removes all routes for a specific lounge
func (r *LoungeRouteRepository) DeleteAllLoungeRoutes(loungeID uuid.UUID) error {
	query := `DELETE FROM lounge_routes WHERE lounge_id = $1`

	_, err := r.db.Exec(query, loungeID)
	if err != nil {
		return fmt.Errorf("failed to delete lounge routes: %w", err)
	}

	return nil
}

// CheckRouteExists checks if a lounge already has a specific route assigned
func (r *LoungeRouteRepository) CheckRouteExists(loungeID, masterRouteID uuid.UUID) (bool, error) {
	var exists bool

	query := `
		SELECT EXISTS(
			SELECT 1 FROM lounge_routes 
			WHERE lounge_id = $1 AND master_route_id = $2
		)
	`

	err := r.db.Get(&exists, query, loungeID, masterRouteID)
	if err != nil {
		return false, fmt.Errorf("failed to check route existence: %w", err)
	}

	return exists, nil
}
