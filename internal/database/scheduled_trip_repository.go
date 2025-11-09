package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// ScheduledTripRepository handles database operations for scheduled_trips table
type ScheduledTripRepository struct {
	db DB
}

// NewScheduledTripRepository creates a new ScheduledTripRepository
func NewScheduledTripRepository(db DB) *ScheduledTripRepository {
	return &ScheduledTripRepository{db: db}
}

// Create creates a new scheduled trip
func (r *ScheduledTripRepository) Create(trip *models.ScheduledTrip) error {
	query := `
		INSERT INTO scheduled_trips (
			id, trip_schedule_id, bus_owner_route_id, permit_id, trip_date, departure_time,
			estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			is_bookable, base_fare, assignment_deadline, status, is_published
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		RETURNING created_at, updated_at
	`

	// Generate ID if not provided
	if trip.ID == "" {
		trip.ID = uuid.New().String()
	}

	err := r.db.QueryRow(
		query,
		trip.ID, trip.TripScheduleID, trip.BusOwnerRouteID, trip.PermitID, trip.TripDate, trip.DepartureTime,
		trip.EstimatedArrivalTime, trip.AssignedDriverID, trip.AssignedConductorID,
		trip.IsBookable, trip.BaseFare, trip.AssignmentDeadline, trip.Status, trip.IsPublished,
	).Scan(&trip.CreatedAt, &trip.UpdatedAt)

	return err
}

// GetByID retrieves a scheduled trip by ID
func (r *ScheduledTripRepository) GetByID(tripID string) (*models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, bus_owner_route_id, permit_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, base_fare, status, cancellation_reason, cancelled_at,
			   assignment_deadline, is_published, created_at, updated_at
		FROM scheduled_trips
		WHERE id = $1
	`

	return r.scanTrip(r.db.QueryRow(query, tripID))
}

// GetByScheduleAndDate checks if a trip exists for a schedule on a specific date
func (r *ScheduledTripRepository) GetByScheduleAndDate(scheduleID string, date time.Time) (*models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, bus_owner_route_id, permit_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, base_fare, status, cancellation_reason, cancelled_at,
			   assignment_deadline, is_published, created_at, updated_at
		FROM scheduled_trips
		WHERE trip_schedule_id = $1 AND trip_date = $2
	`

	return r.scanTrip(r.db.QueryRow(query, scheduleID, date))
}

// GetByScheduleIDsAndDateRange retrieves trips for specific schedule IDs within a date range
func (r *ScheduledTripRepository) GetByScheduleIDsAndDateRange(scheduleIDs []string, startDate, endDate time.Time) ([]models.ScheduledTrip, error) {
	fmt.Printf("🔍 REPO: GetByScheduleIDsAndDateRange called with %d schedule IDs, dates: %s to %s\n",
		len(scheduleIDs), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	if len(scheduleIDs) == 0 {
		fmt.Println("⚠️  REPO: No schedule IDs provided, returning empty array")
		return []models.ScheduledTrip{}, nil
	}

	// Build placeholders for IN clause: $3, $4, $5, ...
	placeholders := make([]string, len(scheduleIDs))
	args := []interface{}{startDate, endDate}
	for i, id := range scheduleIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3) // Start from $3 since $1 and $2 are dates
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT id, trip_schedule_id, bus_owner_route_id, permit_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, base_fare, status, cancellation_reason, cancelled_at,
			   assignment_deadline, is_published, created_at, updated_at
		FROM scheduled_trips
		WHERE trip_schedule_id IN (%s)
		  AND trip_date BETWEEN $1 AND $2
		ORDER BY trip_date, departure_time
	`, strings.Join(placeholders, ", "))

	fmt.Printf("📝 REPO: Executing SQL query:\n%s\n", query)
	fmt.Printf("📝 REPO: Query args: $1=%s, $2=%s, schedule_ids=%v\n",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), scheduleIDs)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		fmt.Printf("❌ REPO: SQL query error: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	fmt.Println("✅ REPO: SQL query executed successfully, scanning results...")
	trips, scanErr := r.scanTrips(rows)
	if scanErr != nil {
		fmt.Printf("❌ REPO: Error scanning trips: %v\n", scanErr)
		return nil, scanErr
	}

	fmt.Printf("✅ REPO: Successfully scanned %d trips from database\n", len(trips))
	return trips, nil
}

// GetByScheduleIDsAndDateRangeWithRouteInfo retrieves trips with route information for specific schedule IDs within a date range
func (r *ScheduledTripRepository) GetByScheduleIDsAndDateRangeWithRouteInfo(scheduleIDs []string, startDate, endDate time.Time) ([]models.ScheduledTripWithRouteInfo, error) {
	fmt.Printf("🔍 REPO: GetByScheduleIDsAndDateRangeWithRouteInfo called with %d schedule IDs, dates: %s to %s\n",
		len(scheduleIDs), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	if len(scheduleIDs) == 0 {
		fmt.Println("⚠️  REPO: No schedule IDs provided, returning empty array")
		return []models.ScheduledTripWithRouteInfo{}, nil
	}

	// Build placeholders for IN clause: $3, $4, $5, ...
	placeholders := make([]string, len(scheduleIDs))
	args := []interface{}{startDate, endDate}
	for i, id := range scheduleIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3) // Start from $3 since $1 and $2 are dates
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT 
			st.id, st.trip_schedule_id, st.permit_id, st.trip_date, st.departure_time,
			st.estimated_arrival_time, st.assigned_driver_id, st.assigned_conductor_id,
			st.is_bookable, st.base_fare, st.status, st.cancellation_reason, st.cancelled_at,
			st.assignment_deadline, st.is_published, st.created_at, st.updated_at,
			mr.route_number, mr.origin_city, mr.destination_city,
			bor.direction
		FROM scheduled_trips st
		LEFT JOIN trip_schedules ts ON st.trip_schedule_id = ts.id
		LEFT JOIN bus_owner_routes bor ON COALESCE(st.bus_owner_route_id, ts.bus_owner_route_id) = bor.id
		LEFT JOIN master_routes mr ON bor.master_route_id = mr.id
		WHERE st.trip_schedule_id IN (%s)
		  AND st.trip_date BETWEEN $1 AND $2
		ORDER BY st.trip_date, st.departure_time
	`, strings.Join(placeholders, ", "))

	fmt.Printf("📝 REPO: Executing SQL query with route info:\n%s\n", query)
	fmt.Printf("📝 REPO: Query args: $1=%s, $2=%s, schedule_ids=%v\n",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), scheduleIDs)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		fmt.Printf("❌ REPO: SQL query error: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	fmt.Println("✅ REPO: SQL query executed successfully, scanning results with route info...")
	trips, scanErr := r.scanTripsWithRouteInfo(rows)
	if scanErr != nil {
		fmt.Printf("❌ REPO: Error scanning trips with route info: %v\n", scanErr)
		return nil, scanErr
	}

	fmt.Printf("✅ REPO: Successfully scanned %d trips with route info from database\n", len(trips))
	return trips, nil
}

// GetByDateRange retrieves scheduled trips within a date range
func (r *ScheduledTripRepository) GetByDateRange(startDate, endDate time.Time) ([]models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, permit_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, base_fare, status, cancellation_reason, cancelled_at,
			   assignment_deadline, is_published, created_at, updated_at
		FROM scheduled_trips
		WHERE trip_date BETWEEN $1 AND $2
		ORDER BY trip_date, departure_time
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTrips(rows)
}

// GetByPermitAndDateRange retrieves scheduled trips for a permit within a date range
func (r *ScheduledTripRepository) GetByPermitAndDateRange(permitID string, startDate, endDate time.Time) ([]models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, permit_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, base_fare, status, cancellation_reason, cancelled_at,
			   assignment_deadline, is_published, created_at, updated_at
		FROM scheduled_trips
		WHERE permit_id = $1 AND trip_date BETWEEN $2 AND $3
		ORDER BY trip_date, departure_time
	`

	rows, err := r.db.Query(query, permitID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTrips(rows)
}

// GetBookableTrips retrieves bookable trips within a date range
func (r *ScheduledTripRepository) GetBookableTrips(startDate, endDate time.Time) ([]models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, permit_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, base_fare, status, cancellation_reason, cancelled_at,
			   assignment_deadline, is_published, created_at, updated_at
		FROM scheduled_trips
		WHERE is_bookable = true
		  AND trip_date BETWEEN $1 AND $2
		  AND status IN ('scheduled', 'confirmed')
		  AND is_published = true
		ORDER BY trip_date, departure_time
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTrips(rows)
}

// Update updates a scheduled trip
func (r *ScheduledTripRepository) Update(trip *models.ScheduledTrip) error {
	query := `
		UPDATE scheduled_trips
		SET assigned_driver_id = $2, assigned_conductor_id = $3,
			status = $4, cancellation_reason = $5, cancelled_at = $6,
			is_published = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		trip.ID, trip.AssignedDriverID, trip.AssignedConductorID,
		trip.Status, trip.CancellationReason, trip.CancelledAt, trip.IsPublished,
	).Scan(&trip.UpdatedAt)

	return err
}

// UpdateSeats - NO LONGER NEEDED (no seat columns in table)
// Seats are managed through bookings table instead

// UpdateStatus updates the status of a scheduled trip
func (r *ScheduledTripRepository) UpdateStatus(tripID string, status models.ScheduledTripStatus) error {
	query := `
		UPDATE scheduled_trips
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(query, tripID, status)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("scheduled trip not found")
	}

	return nil
}

// Cancel cancels a scheduled trip
func (r *ScheduledTripRepository) Cancel(tripID string, reason string) error {
	query := `
		UPDATE scheduled_trips
		SET status = 'cancelled',
			cancellation_reason = $2,
			cancelled_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(query, tripID, reason)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("scheduled trip not found")
	}

	return nil
}

// scanTrip scans a single trip
func (r *ScheduledTripRepository) scanTrip(row scanner) (*models.ScheduledTrip, error) {
	trip := &models.ScheduledTrip{}
	var tripScheduleID, busOwnerRouteID, permitID sql.NullString
	var estimatedArrivalTime sql.NullString
	var assignedDriverID sql.NullString
	var assignedConductorID sql.NullString
	var assignmentDeadline sql.NullTime
	var cancellationReason sql.NullString
	var cancelledAt sql.NullTime

	err := row.Scan(
		&trip.ID,
		&tripScheduleID,
		&busOwnerRouteID,
		&permitID,
		&trip.TripDate,
		&trip.DepartureTime,
		&estimatedArrivalTime,
		&assignedDriverID,
		&assignedConductorID,
		&trip.IsBookable,
		&trip.BaseFare,
		&trip.Status,
		&cancellationReason,
		&cancelledAt,
		&assignmentDeadline,
		&trip.IsPublished,
		&trip.CreatedAt,
		&trip.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types to pointers
	if tripScheduleID.Valid {
		trip.TripScheduleID = &tripScheduleID.String
	}
	if busOwnerRouteID.Valid {
		trip.BusOwnerRouteID = &busOwnerRouteID.String
	}
	if permitID.Valid {
		trip.PermitID = &permitID.String
	}
	if estimatedArrivalTime.Valid {
		trip.EstimatedArrivalTime = &estimatedArrivalTime.String
	}
	if assignedDriverID.Valid {
		trip.AssignedDriverID = &assignedDriverID.String
	}
	if assignedConductorID.Valid {
		trip.AssignedConductorID = &assignedConductorID.String
	}
	if assignmentDeadline.Valid {
		trip.AssignmentDeadline = &assignmentDeadline.Time
	}
	if cancellationReason.Valid {
		trip.CancellationReason = &cancellationReason.String
	}
	if cancelledAt.Valid {
		trip.CancelledAt = &cancelledAt.Time
	}

	return trip, nil
}

// scanTrips scans multiple trips from rows
func (r *ScheduledTripRepository) scanTrips(rows *sql.Rows) ([]models.ScheduledTrip, error) {
	trips := []models.ScheduledTrip{}

	for rows.Next() {
		var trip models.ScheduledTrip
		var tripScheduleID sql.NullString
		var busOwnerRouteID sql.NullString
		var permitID sql.NullString
		var estimatedArrivalTime sql.NullString
		var assignedDriverID sql.NullString
		var assignedConductorID sql.NullString
		var assignmentDeadline sql.NullTime
		var cancellationReason sql.NullString
		var cancelledAt sql.NullTime

		// Must match SELECT order (18 columns):
		// id, trip_schedule_id, bus_owner_route_id, permit_id, trip_date, departure_time,
		// estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
		// is_bookable, base_fare, status, cancellation_reason, cancelled_at,
		// assignment_deadline, is_published, created_at, updated_at
		err := rows.Scan(
			&trip.ID,
			&tripScheduleID,
			&busOwnerRouteID,
			&permitID,
			&trip.TripDate,
			&trip.DepartureTime,
			&estimatedArrivalTime,
			&assignedDriverID,
			&assignedConductorID,
			&trip.IsBookable,
			&trip.BaseFare,
			&trip.Status,
			&cancellationReason,
			&cancelledAt,
			&assignmentDeadline,
			&trip.IsPublished,
			&trip.CreatedAt,
			&trip.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types to pointers
		if tripScheduleID.Valid {
			trip.TripScheduleID = &tripScheduleID.String
		}
		if busOwnerRouteID.Valid {
			trip.BusOwnerRouteID = &busOwnerRouteID.String
		}
		if permitID.Valid {
			trip.PermitID = &permitID.String
		}
		if estimatedArrivalTime.Valid {
			trip.EstimatedArrivalTime = &estimatedArrivalTime.String
		}
		if assignedDriverID.Valid {
			trip.AssignedDriverID = &assignedDriverID.String
		}
		if assignedConductorID.Valid {
			trip.AssignedConductorID = &assignedConductorID.String
		}
		if assignmentDeadline.Valid {
			trip.AssignmentDeadline = &assignmentDeadline.Time
		}
		if cancellationReason.Valid {
			trip.CancellationReason = &cancellationReason.String
		}
		if cancelledAt.Valid {
			trip.CancelledAt = &cancelledAt.Time
		}

		trips = append(trips, trip)
	}

	return trips, rows.Err()
}

// scanTripsWithRouteInfo scans rows into ScheduledTripWithRouteInfo structs
func (r *ScheduledTripRepository) scanTripsWithRouteInfo(rows *sql.Rows) ([]models.ScheduledTripWithRouteInfo, error) {
	trips := []models.ScheduledTripWithRouteInfo{}

	for rows.Next() {
		var tripWithRoute models.ScheduledTripWithRouteInfo
		var tripScheduleID sql.NullString
		var permitID sql.NullString
		var estimatedArrivalTime sql.NullString
		var assignedDriverID sql.NullString
		var assignedConductorID sql.NullString
		var assignmentDeadline sql.NullTime
		var cancellationReason sql.NullString
		var cancelledAt sql.NullTime
		var routeNumber sql.NullString
		var originCity sql.NullString
		var destinationCity sql.NullString
		var direction sql.NullString // "UP" or "DOWN" from database

		// Must match SELECT order (21 columns):
		// st.id, st.trip_schedule_id, st.permit_id, st.trip_date, st.departure_time,
		// st.estimated_arrival_time, st.assigned_driver_id, st.assigned_conductor_id,
		// st.is_bookable, st.base_fare, st.status, st.cancellation_reason, st.cancelled_at,
		// st.assignment_deadline, st.is_published, st.created_at, st.updated_at,
		// mr.route_number, mr.origin_city, mr.destination_city, bor.direction
		err := rows.Scan(
			&tripWithRoute.ID,
			&tripScheduleID,
			&permitID,
			&tripWithRoute.TripDate,
			&tripWithRoute.DepartureTime,
			&estimatedArrivalTime,
			&assignedDriverID,
			&assignedConductorID,
			&tripWithRoute.IsBookable,
			&tripWithRoute.BaseFare,
			&tripWithRoute.Status,
			&cancellationReason,
			&cancelledAt,
			&assignmentDeadline,
			&tripWithRoute.IsPublished,
			&tripWithRoute.CreatedAt,
			&tripWithRoute.UpdatedAt,
			&routeNumber,
			&originCity,
			&destinationCity,
			&direction, // Scan string direction
		)

		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types to pointers for ScheduledTrip fields
		if tripScheduleID.Valid {
			tripWithRoute.TripScheduleID = &tripScheduleID.String
		}
		if permitID.Valid {
			tripWithRoute.PermitID = &permitID.String
		}
		if estimatedArrivalTime.Valid {
			tripWithRoute.EstimatedArrivalTime = &estimatedArrivalTime.String
		}
		if assignedDriverID.Valid {
			tripWithRoute.AssignedDriverID = &assignedDriverID.String
		}
		if assignedConductorID.Valid {
			tripWithRoute.AssignedConductorID = &assignedConductorID.String
		}
		if assignmentDeadline.Valid {
			tripWithRoute.AssignmentDeadline = &assignmentDeadline.Time
		}
		if cancellationReason.Valid {
			tripWithRoute.CancellationReason = &cancellationReason.String
		}
		if cancelledAt.Valid {
			tripWithRoute.CancelledAt = &cancelledAt.Time
		}

		// Convert sql.Null* types to pointers for route info fields
		if routeNumber.Valid {
			tripWithRoute.RouteNumber = &routeNumber.String
		}
		if originCity.Valid {
			tripWithRoute.OriginCity = &originCity.String
		}
		if destinationCity.Valid {
			tripWithRoute.DestinationCity = &destinationCity.String
		}
		if direction.Valid {
			// Convert string "UP"/"DOWN" to boolean for IsUpDirection
			isUp := direction.String == "UP"
			tripWithRoute.IsUpDirection = &isUp
		}

		trips = append(trips, tripWithRoute)
	}

	return trips, rows.Err()
}

// PublishTrip sets is_published to true for a specific trip
func (r *ScheduledTripRepository) PublishTrip(tripID string, busOwnerID string) error {
	query := `
		UPDATE scheduled_trips st
		SET is_published = true, updated_at = NOW()
		FROM trip_schedules ts
		WHERE st.id = $1
		  AND st.trip_schedule_id = ts.id
		  AND ts.bus_owner_id = $2
	`

	result, err := r.db.Exec(query, tripID, busOwnerID)
	if err != nil {
		return fmt.Errorf("failed to publish trip: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("trip not found or unauthorized")
	}

	return nil
}

// UnpublishTrip sets is_published to false for a specific trip
func (r *ScheduledTripRepository) UnpublishTrip(tripID string, busOwnerID string) error {
	query := `
		UPDATE scheduled_trips st
		SET is_published = false, updated_at = NOW()
		FROM trip_schedules ts
		WHERE st.id = $1
		  AND st.trip_schedule_id = ts.id
		  AND ts.bus_owner_id = $2
	`

	result, err := r.db.Exec(query, tripID, busOwnerID)
	if err != nil {
		return fmt.Errorf("failed to unpublish trip: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("trip not found or unauthorized")
	}

	return nil
}

// BulkPublishTrips publishes multiple trips at once
func (r *ScheduledTripRepository) BulkPublishTrips(tripIDs []string, busOwnerID string) (int, error) {
	if len(tripIDs) == 0 {
		return 0, fmt.Errorf("no trip IDs provided")
	}

	query := `
		UPDATE scheduled_trips st
		SET is_published = true, updated_at = NOW()
		FROM trip_schedules ts
		WHERE st.id = ANY($1)
		  AND st.trip_schedule_id = ts.id
		  AND ts.bus_owner_id = $2
	`

	result, err := r.db.Exec(query, tripIDs, busOwnerID)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk publish trips: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// BulkUnpublishTrips unpublishes multiple trips at once
func (r *ScheduledTripRepository) BulkUnpublishTrips(tripIDs []string, busOwnerID string) (int, error) {
	if len(tripIDs) == 0 {
		return 0, fmt.Errorf("no trip IDs provided")
	}

	query := `
		UPDATE scheduled_trips st
		SET is_published = false, updated_at = NOW()
		FROM trip_schedules ts
		WHERE st.id = ANY($1)
		  AND st.trip_schedule_id = ts.id
		  AND ts.bus_owner_id = $2
	`

	result, err := r.db.Exec(query, tripIDs, busOwnerID)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk unpublish trips: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// scanner interface for QueryRow and Rows
type scanner interface {
	Scan(dest ...interface{}) error
}
