package database

import (
	"database/sql"

	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// MasterRouteRepository handles database operations for master_routes table
type MasterRouteRepository struct {
	db DB
}

// NewMasterRouteRepository creates a new MasterRouteRepository
func NewMasterRouteRepository(db DB) *MasterRouteRepository {
	return &MasterRouteRepository{db: db}
}

// GetByID retrieves a master route by ID
func (r *MasterRouteRepository) GetByID(routeID string) (*models.MasterRoute, error) {
	query := `
		SELECT id, route_number, route_name, origin_city, destination_city,
			   total_distance_km, estimated_duration_minutes, encoded_polyline,
			   is_active, created_at, updated_at
		FROM master_routes
		WHERE id = $1
	`

	route := &models.MasterRoute{}
	var totalDistanceKm sql.NullFloat64
	var estimatedDurationMinutes sql.NullInt64
	var encodedPolyline sql.NullString

	err := r.db.QueryRow(query, routeID).Scan(
		&route.ID, &route.RouteNumber, &route.RouteName, &route.OriginCity, &route.DestinationCity,
		&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
		&route.IsActive, &route.CreatedAt, &route.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if totalDistanceKm.Valid {
		route.TotalDistanceKm = &totalDistanceKm.Float64
	}
	if estimatedDurationMinutes.Valid {
		minutes := int(estimatedDurationMinutes.Int64)
		route.EstimatedDurationMinutes = &minutes
	}
	if encodedPolyline.Valid {
		route.EncodedPolyline = &encodedPolyline.String
	}

	return route, nil
}

// GetByRouteNumber retrieves a master route by route number
func (r *MasterRouteRepository) GetByRouteNumber(routeNumber string) (*models.MasterRoute, error) {
	query := `
		SELECT id, route_number, route_name, origin_city, destination_city,
			   total_distance_km, estimated_duration_minutes, encoded_polyline,
			   is_active, created_at, updated_at
		FROM master_routes
		WHERE route_number = $1
	`

	route := &models.MasterRoute{}
	var totalDistanceKm sql.NullFloat64
	var estimatedDurationMinutes sql.NullInt64
	var encodedPolyline sql.NullString

	err := r.db.QueryRow(query, routeNumber).Scan(
		&route.ID, &route.RouteNumber, &route.RouteName, &route.OriginCity, &route.DestinationCity,
		&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
		&route.IsActive, &route.CreatedAt, &route.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if totalDistanceKm.Valid {
		route.TotalDistanceKm = &totalDistanceKm.Float64
	}
	if estimatedDurationMinutes.Valid {
		minutes := int(estimatedDurationMinutes.Int64)
		route.EstimatedDurationMinutes = &minutes
	}
	if encodedPolyline.Valid {
		route.EncodedPolyline = &encodedPolyline.String
	}

	return route, nil
}

// GetAll retrieves all master routes
func (r *MasterRouteRepository) GetAll(activeOnly bool) ([]models.MasterRoute, error) {
	query := `
		SELECT id, route_number, route_name, origin_city, destination_city,
			   total_distance_km, estimated_duration_minutes, encoded_polyline,
			   is_active, created_at, updated_at
		FROM master_routes
	`

	if activeOnly {
		query += " WHERE is_active = true"
	}

	query += " ORDER BY route_number"

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := []models.MasterRoute{}
	for rows.Next() {
		var route models.MasterRoute
		var totalDistanceKm sql.NullFloat64
		var estimatedDurationMinutes sql.NullInt64
		var encodedPolyline sql.NullString

		err := rows.Scan(
			&route.ID, &route.RouteNumber, &route.RouteName, &route.OriginCity, &route.DestinationCity,
			&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
			&route.IsActive, &route.CreatedAt, &route.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if totalDistanceKm.Valid {
			route.TotalDistanceKm = &totalDistanceKm.Float64
		}
		if estimatedDurationMinutes.Valid {
			minutes := int(estimatedDurationMinutes.Int64)
			route.EstimatedDurationMinutes = &minutes
		}
		if encodedPolyline.Valid {
			route.EncodedPolyline = &encodedPolyline.String
		}

		routes = append(routes, route)
	}

	return routes, rows.Err()
}

// GetStopsByRouteID retrieves all stops for a master route
func (r *MasterRouteRepository) GetStopsByRouteID(routeID string) ([]models.MasterRouteStop, error) {
	query := `
		SELECT id, master_route_id, stop_name, stop_order,
			   latitude, longitude, arrival_time_offset_minutes,
			   is_major_stop, created_at
		FROM master_route_stops
		WHERE master_route_id = $1
		ORDER BY stop_order
	`

	rows, err := r.db.Query(query, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stops := []models.MasterRouteStop{}
	for rows.Next() {
		var stop models.MasterRouteStop
		var latitude sql.NullFloat64
		var longitude sql.NullFloat64
		var arrivalTimeOffsetMinutes sql.NullInt64

		err := rows.Scan(
			&stop.ID, &stop.MasterRouteID, &stop.StopName, &stop.StopOrder,
			&latitude, &longitude, &arrivalTimeOffsetMinutes,
			&stop.IsMajorStop, &stop.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if latitude.Valid {
			stop.Latitude = &latitude.Float64
		}
		if longitude.Valid {
			stop.Longitude = &longitude.Float64
		}
		if arrivalTimeOffsetMinutes.Valid {
			minutes := int(arrivalTimeOffsetMinutes.Int64)
			stop.ArrivalTimeOffsetMinutes = &minutes
		}

		stops = append(stops, stop)
	}

	return stops, rows.Err()
}

// GetStopByID retrieves a specific stop by ID
func (r *MasterRouteRepository) GetStopByID(stopID string) (*models.MasterRouteStop, error) {
	query := `
		SELECT id, master_route_id, stop_name, stop_order,
			   latitude, longitude, arrival_time_offset_minutes,
			   is_major_stop, created_at
		FROM master_route_stops
		WHERE id = $1
	`

	stop := &models.MasterRouteStop{}
	var latitude sql.NullFloat64
	var longitude sql.NullFloat64
	var arrivalTimeOffsetMinutes sql.NullInt64

	err := r.db.QueryRow(query, stopID).Scan(
		&stop.ID, &stop.MasterRouteID, &stop.StopName, &stop.StopOrder,
		&latitude, &longitude, &arrivalTimeOffsetMinutes,
		&stop.IsMajorStop, &stop.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if latitude.Valid {
		stop.Latitude = &latitude.Float64
	}
	if longitude.Valid {
		stop.Longitude = &longitude.Float64
	}
	if arrivalTimeOffsetMinutes.Valid {
		minutes := int(arrivalTimeOffsetMinutes.Int64)
		stop.ArrivalTimeOffsetMinutes = &minutes
	}

	return stop, nil
}
