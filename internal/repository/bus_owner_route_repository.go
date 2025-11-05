package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type BusOwnerRouteRepository struct {
	db *sqlx.DB
}

func NewBusOwnerRouteRepository(db *sqlx.DB) *BusOwnerRouteRepository {
	return &BusOwnerRouteRepository{db: db}
}

// Create creates a new bus owner route
func (r *BusOwnerRouteRepository) Create(route *models.BusOwnerRoute) error {
	query := `
		INSERT INTO bus_owner_routes (
			id, bus_owner_id, master_route_id, custom_route_name,
			direction, selected_stop_ids, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NOW(), NOW()
		)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		route.ID,
		route.BusOwnerID,
		route.MasterRouteID,
		route.CustomRouteName,
		route.Direction,
		pq.Array(route.SelectedStopIDs),
	).Scan(&route.CreatedAt, &route.UpdatedAt)

	return err
}

// GetByID retrieves a bus owner route by ID
func (r *BusOwnerRouteRepository) GetByID(id string) (*models.BusOwnerRoute, error) {
	var route models.BusOwnerRoute
	query := `
		SELECT id, bus_owner_id, master_route_id, custom_route_name,
			   direction, selected_stop_ids, created_at, updated_at
		FROM bus_owner_routes
		WHERE id = $1
	`

	err := r.db.Get(&route, query, id)
	if err != nil {
		return nil, err
	}

	return &route, nil
}

// GetByBusOwnerID retrieves all routes for a bus owner
func (r *BusOwnerRouteRepository) GetByBusOwnerID(busOwnerID string) ([]models.BusOwnerRoute, error) {
	var routes []models.BusOwnerRoute
	query := `
		SELECT id, bus_owner_id, master_route_id, custom_route_name,
			   direction, selected_stop_ids, created_at, updated_at
		FROM bus_owner_routes
		WHERE bus_owner_id = $1
		ORDER BY created_at DESC
	`

	err := r.db.Select(&routes, query, busOwnerID)
	if err != nil {
		return nil, err
	}

	return routes, nil
}

// GetByMasterRouteID retrieves all custom routes for a specific master route
func (r *BusOwnerRouteRepository) GetByMasterRouteID(busOwnerID, masterRouteID string) ([]models.BusOwnerRoute, error) {
	var routes []models.BusOwnerRoute
	query := `
		SELECT id, bus_owner_id, master_route_id, custom_route_name,
			   direction, selected_stop_ids, created_at, updated_at
		FROM bus_owner_routes
		WHERE bus_owner_id = $1 AND master_route_id = $2
		ORDER BY direction, created_at DESC
	`

	err := r.db.Select(&routes, query, busOwnerID, masterRouteID)
	if err != nil {
		return nil, err
	}

	return routes, nil
}

// Update updates an existing bus owner route
func (r *BusOwnerRouteRepository) Update(route *models.BusOwnerRoute) error {
	query := `
		UPDATE bus_owner_routes
		SET custom_route_name = $1,
			selected_stop_ids = $2,
			updated_at = NOW()
		WHERE id = $3 AND bus_owner_id = $4
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		route.CustomRouteName,
		pq.Array(route.SelectedStopIDs),
		route.ID,
		route.BusOwnerID,
	).Scan(&route.UpdatedAt)

	if err == sql.ErrNoRows {
		return fmt.Errorf("route not found or unauthorized")
	}

	return err
}

// Delete deletes a bus owner route
func (r *BusOwnerRouteRepository) Delete(id, busOwnerID string) error {
	query := `DELETE FROM bus_owner_routes WHERE id = $1 AND bus_owner_id = $2`

	result, err := r.db.Exec(query, id, busOwnerID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("route not found or unauthorized")
	}

	return nil
}

// ValidateStopsExist validates that all selected stop IDs exist in the master route
func (r *BusOwnerRouteRepository) ValidateStopsExist(masterRouteID string, stopIDs []string) (bool, error) {
	query := `
		SELECT COUNT(DISTINCT id)
		FROM master_route_stops
		WHERE master_route_id = $1 AND id = ANY($2)
	`

	var count int
	err := r.db.Get(&count, query, masterRouteID, pq.Array(stopIDs))
	if err != nil {
		return false, err
	}

	return count == len(stopIDs), nil
}

// ValidateFirstAndLastStops validates that first and last stops of master route are included
func (r *BusOwnerRouteRepository) ValidateFirstAndLastStops(masterRouteID string, stopIDs []string) (bool, error) {
	query := `
		SELECT id
		FROM master_route_stops
		WHERE master_route_id = $1
		ORDER BY stop_order ASC
		LIMIT 1
	`

	var firstStopID string
	err := r.db.Get(&firstStopID, query, masterRouteID)
	if err != nil {
		return false, err
	}

	query = `
		SELECT id
		FROM master_route_stops
		WHERE master_route_id = $1
		ORDER BY stop_order DESC
		LIMIT 1
	`

	var lastStopID string
	err = r.db.Get(&lastStopID, query, masterRouteID)
	if err != nil {
		return false, err
	}

	// Check if both first and last stops are in the selected stops
	hasFirst := false
	hasLast := false

	for _, stopID := range stopIDs {
		if stopID == firstStopID {
			hasFirst = true
		}
		if stopID == lastStopID {
			hasLast = true
		}
	}

	return hasFirst && hasLast, nil
}
