package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// RoutePermitRepository handles database operations for route_permits table
type RoutePermitRepository struct {
	db DB
}

// NewRoutePermitRepository creates a new RoutePermitRepository
func NewRoutePermitRepository(db DB) *RoutePermitRepository {
	return &RoutePermitRepository{db: db}
}

// Create creates a new route permit
func (r *RoutePermitRepository) Create(permit *models.RoutePermit) error {
	query := `
		INSERT INTO route_permits (
			id, bus_owner_id, permit_number, bus_registration_number,
			master_route_id, route_number, route_name, full_origin_city,
			full_destination_city, via, total_distance_km, estimated_duration_minutes,
			issue_date, expiry_date, permit_type, approved_fare, max_trips_per_day,
			allowed_bus_types, restrictions, status, verified_at, permit_document_url
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		permit.ID, permit.BusOwnerID, permit.PermitNumber, permit.BusRegistrationNumber,
		permit.MasterRouteID, permit.RouteNumber, permit.RouteName, permit.FullOriginCity,
		permit.FullDestinationCity, permit.Via, permit.TotalDistanceKm, permit.EstimatedDurationMinutes,
		permit.IssueDate, permit.ExpiryDate, permit.PermitType, permit.ApprovedFare, permit.MaxTripsPerDay,
		permit.AllowedBusTypes, permit.Restrictions, permit.Status, permit.VerifiedAt, permit.PermitDocumentURL,
	).Scan(&permit.CreatedAt, &permit.UpdatedAt)

	return err
}

// GetByID retrieves a route permit by ID
func (r *RoutePermitRepository) GetByID(permitID string) (*models.RoutePermit, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_number, bus_registration_number,
			master_route_id, route_number, route_name, full_origin_city,
			full_destination_city, via, total_distance_km, estimated_duration_minutes,
			issue_date, expiry_date, permit_type, approved_fare, max_trips_per_day,
			allowed_bus_types, restrictions, status, verified_at, permit_document_url,
			created_at, updated_at
		FROM route_permits
		WHERE id = $1
	`

	permit := &models.RoutePermit{}
	err := r.db.QueryRow(query, permitID).Scan(
		&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
		&permit.MasterRouteID, &permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity,
		&permit.FullDestinationCity, &permit.Via, &permit.TotalDistanceKm, &permit.EstimatedDurationMinutes,
		&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.MaxTripsPerDay,
		&permit.AllowedBusTypes, &permit.Restrictions, &permit.Status, &permit.VerifiedAt, &permit.PermitDocumentURL,
		&permit.CreatedAt, &permit.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return permit, nil
}

// GetByOwnerID retrieves all permits for a bus owner
func (r *RoutePermitRepository) GetByOwnerID(busOwnerID string) ([]models.RoutePermit, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_number, bus_registration_number,
			master_route_id, route_number, route_name, full_origin_city,
			full_destination_city, via, total_distance_km, estimated_duration_minutes,
			issue_date, expiry_date, permit_type, approved_fare, max_trips_per_day,
			allowed_bus_types, restrictions, status, verified_at, permit_document_url,
			created_at, updated_at
		FROM route_permits
		WHERE bus_owner_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permits := []models.RoutePermit{}
	for rows.Next() {
		var permit models.RoutePermit
		err := rows.Scan(
			&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
			&permit.MasterRouteID, &permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity,
			&permit.FullDestinationCity, &permit.Via, &permit.TotalDistanceKm, &permit.EstimatedDurationMinutes,
			&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.MaxTripsPerDay,
			&permit.AllowedBusTypes, &permit.Restrictions, &permit.Status, &permit.VerifiedAt, &permit.PermitDocumentURL,
			&permit.CreatedAt, &permit.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		permits = append(permits, permit)
	}

	return permits, nil
}

// GetByPermitNumber retrieves a permit by permit number
func (r *RoutePermitRepository) GetByPermitNumber(permitNumber string, busOwnerID string) (*models.RoutePermit, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_number, bus_registration_number,
			master_route_id, route_number, route_name, full_origin_city,
			full_destination_city, via, total_distance_km, estimated_duration_minutes,
			issue_date, expiry_date, permit_type, approved_fare, max_trips_per_day,
			allowed_bus_types, restrictions, status, verified_at, permit_document_url,
			created_at, updated_at
		FROM route_permits
		WHERE permit_number = $1 AND bus_owner_id = $2
	`

	permit := &models.RoutePermit{}
	err := r.db.QueryRow(query, permitNumber, busOwnerID).Scan(
		&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
		&permit.MasterRouteID, &permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity,
		&permit.FullDestinationCity, &permit.Via, &permit.TotalDistanceKm, &permit.EstimatedDurationMinutes,
		&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.MaxTripsPerDay,
		&permit.AllowedBusTypes, &permit.Restrictions, &permit.Status, &permit.VerifiedAt, &permit.PermitDocumentURL,
		&permit.CreatedAt, &permit.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return permit, nil
}

// GetByBusRegistration retrieves a permit by bus registration number
func (r *RoutePermitRepository) GetByBusRegistration(busRegistration string, busOwnerID string) (*models.RoutePermit, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_number, bus_registration_number,
			master_route_id, route_number, route_name, full_origin_city,
			full_destination_city, via, total_distance_km, estimated_duration_minutes,
			issue_date, expiry_date, permit_type, approved_fare, max_trips_per_day,
			allowed_bus_types, restrictions, status, verified_at, permit_document_url,
			created_at, updated_at
		FROM route_permits
		WHERE bus_registration_number = $1 AND bus_owner_id = $2
	`

	permit := &models.RoutePermit{}
	err := r.db.QueryRow(query, busRegistration, busOwnerID).Scan(
		&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
		&permit.MasterRouteID, &permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity,
		&permit.FullDestinationCity, &permit.Via, &permit.TotalDistanceKm, &permit.EstimatedDurationMinutes,
		&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.MaxTripsPerDay,
		&permit.AllowedBusTypes, &permit.Restrictions, &permit.Status, &permit.VerifiedAt, &permit.PermitDocumentURL,
		&permit.CreatedAt, &permit.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil if not found (not an error for checking)
		}
		return nil, err
	}

	return permit, nil
}

// Update updates a route permit
func (r *RoutePermitRepository) Update(permitID string, req *models.UpdateRoutePermitRequest) error {
	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.BusRegistrationNumber != nil {
		updates = append(updates, fmt.Sprintf("bus_registration_number = $%d", argCount))
		args = append(args, *req.BusRegistrationNumber)
		argCount++
	}

	if req.Via != nil {
		// Parse comma-separated string into array
		viaArray := strings.Split(*req.Via, ",")
		for i := range viaArray {
			viaArray[i] = strings.TrimSpace(viaArray[i])
		}
		updates = append(updates, fmt.Sprintf("via = $%d", argCount))
		args = append(args, models.StringArray(viaArray))
		argCount++
	}

	if req.ApprovedFare != nil {
		updates = append(updates, fmt.Sprintf("approved_fare = $%d", argCount))
		args = append(args, *req.ApprovedFare)
		argCount++
	}

	if req.ValidityTo != nil {
		expiryDate, err := time.Parse("2006-01-02", *req.ValidityTo)
		if err != nil {
			return fmt.Errorf("invalid validity_to format")
		}
		updates = append(updates, fmt.Sprintf("expiry_date = $%d", argCount))
		args = append(args, expiryDate)
		argCount++
	}

	if req.TotalDistanceKm != nil {
		updates = append(updates, fmt.Sprintf("total_distance_km = $%d", argCount))
		args = append(args, *req.TotalDistanceKm)
		argCount++
	}

	if req.EstimatedDuration != nil {
		updates = append(updates, fmt.Sprintf("estimated_duration_minutes = $%d", argCount))
		args = append(args, *req.EstimatedDuration)
		argCount++
	}

	if req.MaxTripsPerDay != nil {
		updates = append(updates, fmt.Sprintf("max_trips_per_day = $%d", argCount))
		args = append(args, *req.MaxTripsPerDay)
		argCount++
	}

	if req.AllowedBusTypes != nil {
		updates = append(updates, fmt.Sprintf("allowed_bus_types = $%d", argCount))
		args = append(args, models.StringArray(req.AllowedBusTypes))
		argCount++
	}

	if req.Restrictions != nil {
		updates = append(updates, fmt.Sprintf("restrictions = $%d", argCount))
		args = append(args, *req.Restrictions)
		argCount++
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Add updated_at
	updates = append(updates, "updated_at = NOW()")

	// Add permit ID to args
	args = append(args, permitID)

	query := fmt.Sprintf(`
		UPDATE route_permits
		SET %s
		WHERE id = $%d
	`, strings.Join(updates, ", "), argCount)

	_, err := r.db.Exec(query, args...)
	return err
}

// Delete deletes a route permit
func (r *RoutePermitRepository) Delete(permitID string, busOwnerID string) error {
	query := `DELETE FROM route_permits WHERE id = $1 AND bus_owner_id = $2`
	result, err := r.db.Exec(query, permitID, busOwnerID)
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

// GetValidPermits retrieves all valid permits for a bus owner
func (r *RoutePermitRepository) GetValidPermits(busOwnerID string) ([]models.RoutePermit, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_number, bus_registration_number,
			master_route_id, route_number, route_name, full_origin_city,
			full_destination_city, via, total_distance_km, estimated_duration_minutes,
			issue_date, expiry_date, permit_type, approved_fare, max_trips_per_day,
			allowed_bus_types, restrictions, status, verified_at, permit_document_url,
			created_at, updated_at
		FROM route_permits
		WHERE bus_owner_id = $1
		  AND status = 'verified'
		  AND expiry_date >= CURRENT_DATE
		ORDER BY expiry_date ASC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permits := []models.RoutePermit{}
	for rows.Next() {
		var permit models.RoutePermit
		err := rows.Scan(
			&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
			&permit.MasterRouteID, &permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity,
			&permit.FullDestinationCity, &permit.Via, &permit.TotalDistanceKm, &permit.EstimatedDurationMinutes,
			&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.MaxTripsPerDay,
			&permit.AllowedBusTypes, &permit.Restrictions, &permit.Status, &permit.VerifiedAt, &permit.PermitDocumentURL,
			&permit.CreatedAt, &permit.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		permits = append(permits, permit)
	}

	return permits, nil
}

// CountPermits returns the count of permits for a bus owner
func (r *RoutePermitRepository) CountPermits(busOwnerID string) (int, error) {
	query := `SELECT COUNT(*) FROM route_permits WHERE bus_owner_id = $1`
	var count int
	err := r.db.QueryRow(query, busOwnerID).Scan(&count)
	return count, err
}
