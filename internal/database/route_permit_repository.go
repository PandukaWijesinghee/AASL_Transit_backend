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
			master_route_id, via,
			issue_date, expiry_date, permit_type, approved_fare, max_trips_per_day,
			allowed_bus_types, restrictions, status, verified_at, permit_document_url
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		permit.ID, permit.BusOwnerID, permit.PermitNumber, permit.BusRegistrationNumber,
		permit.MasterRouteID, permit.Via,
		permit.IssueDate, permit.ExpiryDate, permit.PermitType, permit.ApprovedFare, permit.MaxTripsPerDay,
		permit.AllowedBusTypes, permit.Restrictions, permit.Status, permit.VerifiedAt, permit.PermitDocumentURL,
	).Scan(&permit.CreatedAt, &permit.UpdatedAt)

	return err
}

// GetByID retrieves a route permit by ID with route details from master_routes
func (r *RoutePermitRepository) GetByID(permitID string) (*models.RoutePermitWithDetails, error) {
	query := `
		SELECT
			rp.id, rp.bus_owner_id, rp.permit_number, rp.bus_registration_number,
			rp.master_route_id, rp.via,
			rp.issue_date, rp.expiry_date, rp.permit_type, rp.approved_fare, rp.approved_seating_capacity, rp.max_trips_per_day,
			rp.allowed_bus_types, rp.restrictions, rp.status, rp.verified_at, rp.permit_document_url,
			rp.created_at, rp.updated_at,
			mr.route_number, mr.route_name, mr.origin_city, mr.destination_city,
			mr.total_distance_km, mr.estimated_duration_minutes, mr.encoded_polyline
		FROM route_permits rp
		JOIN master_routes mr ON rp.master_route_id = mr.id
		WHERE rp.id = $1
	`

	permit := &models.RoutePermitWithDetails{}
	var maxTripsPerDay sql.NullInt64
	var restrictions sql.NullString
	var verifiedAt sql.NullTime
	var permitDocumentURL sql.NullString
	var via models.StringArray
	var allowedBusTypes models.StringArray
	var totalDistanceKm sql.NullFloat64
	var estimatedDurationMinutes sql.NullInt64
	var encodedPolyline sql.NullString

	err := r.db.QueryRow(query, permitID).Scan(
		&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
		&permit.MasterRouteID, &via,
		&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.ApprovedSeatingCapacity, &maxTripsPerDay,
		&allowedBusTypes, &restrictions, &permit.Status, &verifiedAt, &permitDocumentURL,
		&permit.CreatedAt, &permit.UpdatedAt,
		&permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity, &permit.FullDestinationCity,
		&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
	)

	if err != nil {
		return nil, err
	}

	// Assign arrays (they'll be nil if NULL in database)
	permit.Via = via
	permit.AllowedBusTypes = allowedBusTypes

	// Convert sql.Null* types to pointers
	if maxTripsPerDay.Valid {
		trips := int(maxTripsPerDay.Int64)
		permit.MaxTripsPerDay = &trips
	}
	if restrictions.Valid {
		permit.Restrictions = &restrictions.String
	}
	if verifiedAt.Valid {
		permit.VerifiedAt = &verifiedAt.Time
	}
	if permitDocumentURL.Valid {
		permit.PermitDocumentURL = &permitDocumentURL.String
	}
	if totalDistanceKm.Valid {
		permit.TotalDistanceKm = &totalDistanceKm.Float64
	}
	if estimatedDurationMinutes.Valid {
		minutes := int(estimatedDurationMinutes.Int64)
		permit.EstimatedDurationMinutes = &minutes
	}
	if encodedPolyline.Valid {
		permit.EncodedPolyline = &encodedPolyline.String
	}

	return permit, nil
}

// GetByOwnerID retrieves all permits for a bus owner with route details
func (r *RoutePermitRepository) GetByOwnerID(busOwnerID string) ([]models.RoutePermitWithDetails, error) {
	query := `
		SELECT
			rp.id, rp.bus_owner_id, rp.permit_number, rp.bus_registration_number,
			rp.master_route_id, rp.via,
			rp.issue_date, rp.expiry_date, rp.permit_type, rp.approved_fare, rp.approved_seating_capacity, rp.max_trips_per_day,
			rp.allowed_bus_types, rp.restrictions, rp.status, rp.verified_at, rp.permit_document_url,
			rp.created_at, rp.updated_at,
			mr.route_number, mr.route_name, mr.origin_city, mr.destination_city,
			mr.total_distance_km, mr.estimated_duration_minutes, mr.encoded_polyline
		FROM route_permits rp
		JOIN master_routes mr ON rp.master_route_id = mr.id
		WHERE rp.bus_owner_id = $1
		ORDER BY rp.created_at DESC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permits := []models.RoutePermitWithDetails{}
	for rows.Next() {
		var permit models.RoutePermitWithDetails
		var maxTripsPerDay sql.NullInt64
		var restrictions sql.NullString
		var verifiedAt sql.NullTime
		var permitDocumentURL sql.NullString
		var via models.StringArray
		var allowedBusTypes models.StringArray
		var totalDistanceKm sql.NullFloat64
		var estimatedDurationMinutes sql.NullInt64
		var encodedPolyline sql.NullString

		err := rows.Scan(
			&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
			&permit.MasterRouteID, &via,
			&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.ApprovedSeatingCapacity, &maxTripsPerDay,
			&allowedBusTypes, &restrictions, &permit.Status, &verifiedAt, &permitDocumentURL,
			&permit.CreatedAt, &permit.UpdatedAt,
			&permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity, &permit.FullDestinationCity,
			&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
		)
		if err != nil {
			return nil, err
		}

		// Assign arrays
		permit.Via = via
		permit.AllowedBusTypes = allowedBusTypes

		// Convert sql.Null* types to pointers
		if maxTripsPerDay.Valid {
			trips := int(maxTripsPerDay.Int64)
			permit.MaxTripsPerDay = &trips
		}
		if restrictions.Valid {
			permit.Restrictions = &restrictions.String
		}
		if verifiedAt.Valid {
			permit.VerifiedAt = &verifiedAt.Time
		}
		if permitDocumentURL.Valid {
			permit.PermitDocumentURL = &permitDocumentURL.String
		}
		if totalDistanceKm.Valid {
			permit.TotalDistanceKm = &totalDistanceKm.Float64
		}
		if estimatedDurationMinutes.Valid {
			minutes := int(estimatedDurationMinutes.Int64)
			permit.EstimatedDurationMinutes = &minutes
		}
		if encodedPolyline.Valid {
			permit.EncodedPolyline = &encodedPolyline.String
		}

		permits = append(permits, permit)
	}

	return permits, nil
}

// GetByPermitNumber retrieves a permit by permit number with route details
func (r *RoutePermitRepository) GetByPermitNumber(permitNumber string, busOwnerID string) (*models.RoutePermitWithDetails, error) {
	query := `
		SELECT
			rp.id, rp.bus_owner_id, rp.permit_number, rp.bus_registration_number,
			rp.master_route_id, rp.via,
			rp.issue_date, rp.expiry_date, rp.permit_type, rp.approved_fare, rp.approved_seating_capacity, rp.max_trips_per_day,
			rp.allowed_bus_types, rp.restrictions, rp.status, rp.verified_at, rp.permit_document_url,
			rp.created_at, rp.updated_at,
			mr.route_number, mr.route_name, mr.origin_city, mr.destination_city,
			mr.total_distance_km, mr.estimated_duration_minutes, mr.encoded_polyline
		FROM route_permits rp
		JOIN master_routes mr ON rp.master_route_id = mr.id
		WHERE rp.permit_number = $1 AND rp.bus_owner_id = $2
	`

	permit := &models.RoutePermitWithDetails{}
	var maxTripsPerDay sql.NullInt64
	var restrictions sql.NullString
	var verifiedAt sql.NullTime
	var permitDocumentURL sql.NullString
	var via models.StringArray
	var allowedBusTypes models.StringArray
	var totalDistanceKm sql.NullFloat64
	var estimatedDurationMinutes sql.NullInt64
	var encodedPolyline sql.NullString

	err := r.db.QueryRow(query, permitNumber, busOwnerID).Scan(
		&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
		&permit.MasterRouteID, &via,
		&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.ApprovedSeatingCapacity, &maxTripsPerDay,
		&allowedBusTypes, &restrictions, &permit.Status, &verifiedAt, &permitDocumentURL,
		&permit.CreatedAt, &permit.UpdatedAt,
		&permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity, &permit.FullDestinationCity,
		&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
	)

	if err != nil {
		return nil, err
	}

	// Assign arrays
	permit.Via = via
	permit.AllowedBusTypes = allowedBusTypes

	// Convert sql.Null* types to pointers
	if maxTripsPerDay.Valid {
		trips := int(maxTripsPerDay.Int64)
		permit.MaxTripsPerDay = &trips
	}
	if restrictions.Valid {
		permit.Restrictions = &restrictions.String
	}
	if verifiedAt.Valid {
		permit.VerifiedAt = &verifiedAt.Time
	}
	if permitDocumentURL.Valid {
		permit.PermitDocumentURL = &permitDocumentURL.String
	}
	if totalDistanceKm.Valid {
		permit.TotalDistanceKm = &totalDistanceKm.Float64
	}
	if estimatedDurationMinutes.Valid {
		minutes := int(estimatedDurationMinutes.Int64)
		permit.EstimatedDurationMinutes = &minutes
	}
	if encodedPolyline.Valid {
		permit.EncodedPolyline = &encodedPolyline.String
	}

	return permit, nil
}

// GetByBusRegistration retrieves a permit by bus registration number with route details
func (r *RoutePermitRepository) GetByBusRegistration(busRegistration string, busOwnerID string) (*models.RoutePermitWithDetails, error) {
	query := `
		SELECT
			rp.id, rp.bus_owner_id, rp.permit_number, rp.bus_registration_number,
			rp.master_route_id, rp.via,
			rp.issue_date, rp.expiry_date, rp.permit_type, rp.approved_fare, rp.approved_seating_capacity, rp.max_trips_per_day,
			rp.allowed_bus_types, rp.restrictions, rp.status, rp.verified_at, rp.permit_document_url,
			rp.created_at, rp.updated_at,
			mr.route_number, mr.route_name, mr.origin_city, mr.destination_city,
			mr.total_distance_km, mr.estimated_duration_minutes, mr.encoded_polyline
		FROM route_permits rp
		JOIN master_routes mr ON rp.master_route_id = mr.id
		WHERE rp.bus_registration_number = $1 AND rp.bus_owner_id = $2
	`

	permit := &models.RoutePermitWithDetails{}
	var maxTripsPerDay sql.NullInt64
	var restrictions sql.NullString
	var verifiedAt sql.NullTime
	var permitDocumentURL sql.NullString
	var via models.StringArray
	var allowedBusTypes models.StringArray
	var totalDistanceKm sql.NullFloat64
	var estimatedDurationMinutes sql.NullInt64
	var encodedPolyline sql.NullString

	err := r.db.QueryRow(query, busRegistration, busOwnerID).Scan(
		&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
		&permit.MasterRouteID, &via,
		&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.ApprovedSeatingCapacity, &maxTripsPerDay,
		&allowedBusTypes, &restrictions, &permit.Status, &verifiedAt, &permitDocumentURL,
		&permit.CreatedAt, &permit.UpdatedAt,
		&permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity, &permit.FullDestinationCity,
		&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil if not found (not an error for checking)
		}
		return nil, err
	}

	// Assign arrays
	permit.Via = via
	permit.AllowedBusTypes = allowedBusTypes

	// Convert sql.Null* types to pointers
	if maxTripsPerDay.Valid {
		trips := int(maxTripsPerDay.Int64)
		permit.MaxTripsPerDay = &trips
	}
	if restrictions.Valid {
		permit.Restrictions = &restrictions.String
	}
	if verifiedAt.Valid {
		permit.VerifiedAt = &verifiedAt.Time
	}
	if permitDocumentURL.Valid {
		permit.PermitDocumentURL = &permitDocumentURL.String
	}
	if totalDistanceKm.Valid {
		permit.TotalDistanceKm = &totalDistanceKm.Float64
	}
	if estimatedDurationMinutes.Valid {
		minutes := int(estimatedDurationMinutes.Int64)
		permit.EstimatedDurationMinutes = &minutes
	}
	if encodedPolyline.Valid {
		permit.EncodedPolyline = &encodedPolyline.String
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

// GetValidPermits retrieves all valid permits for a bus owner with route details
func (r *RoutePermitRepository) GetValidPermits(busOwnerID string) ([]models.RoutePermitWithDetails, error) {
	query := `
		SELECT
			rp.id, rp.bus_owner_id, rp.permit_number, rp.bus_registration_number,
			rp.master_route_id, rp.via,
			rp.issue_date, rp.expiry_date, rp.permit_type, rp.approved_fare, rp.approved_seating_capacity, rp.max_trips_per_day,
			rp.allowed_bus_types, rp.restrictions, rp.status, rp.verified_at, rp.permit_document_url,
			rp.created_at, rp.updated_at,
			mr.route_number, mr.route_name, mr.origin_city, mr.destination_city,
			mr.total_distance_km, mr.estimated_duration_minutes, mr.encoded_polyline
		FROM route_permits rp
		JOIN master_routes mr ON rp.master_route_id = mr.id
		WHERE rp.bus_owner_id = $1
		  AND rp.status = 'verified'
		  AND rp.expiry_date >= CURRENT_DATE
		ORDER BY rp.expiry_date ASC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permits := []models.RoutePermitWithDetails{}
	for rows.Next() {
		var permit models.RoutePermitWithDetails
		var maxTripsPerDay sql.NullInt64
		var restrictions sql.NullString
		var verifiedAt sql.NullTime
		var permitDocumentURL sql.NullString
		var via models.StringArray
		var allowedBusTypes models.StringArray
		var totalDistanceKm sql.NullFloat64
		var estimatedDurationMinutes sql.NullInt64
		var encodedPolyline sql.NullString

		err := rows.Scan(
			&permit.ID, &permit.BusOwnerID, &permit.PermitNumber, &permit.BusRegistrationNumber,
			&permit.MasterRouteID, &via,
			&permit.IssueDate, &permit.ExpiryDate, &permit.PermitType, &permit.ApprovedFare, &permit.ApprovedSeatingCapacity, &maxTripsPerDay,
			&allowedBusTypes, &restrictions, &permit.Status, &verifiedAt, &permitDocumentURL,
			&permit.CreatedAt, &permit.UpdatedAt,
			&permit.RouteNumber, &permit.RouteName, &permit.FullOriginCity, &permit.FullDestinationCity,
			&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
		)
		if err != nil {
			return nil, err
		}

		// Assign arrays (they'll be nil if NULL in database)
		permit.Via = via
		permit.AllowedBusTypes = allowedBusTypes

		// Convert sql.Null* types to pointers
		if maxTripsPerDay.Valid {
			trips := int(maxTripsPerDay.Int64)
			permit.MaxTripsPerDay = &trips
		}
		if restrictions.Valid {
			permit.Restrictions = &restrictions.String
		}
		if verifiedAt.Valid {
			permit.VerifiedAt = &verifiedAt.Time
		}
		if permitDocumentURL.Valid {
			permit.PermitDocumentURL = &permitDocumentURL.String
		}
		if totalDistanceKm.Valid {
			permit.TotalDistanceKm = &totalDistanceKm.Float64
		}
		if estimatedDurationMinutes.Valid {
			minutes := int(estimatedDurationMinutes.Int64)
			permit.EstimatedDurationMinutes = &minutes
		}
		if encodedPolyline.Valid {
			permit.EncodedPolyline = &encodedPolyline.String
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
