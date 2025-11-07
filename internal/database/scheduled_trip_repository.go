package database

import (
	"database/sql"
	"fmt"
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
			id, trip_schedule_id, custom_route_id, permit_id, bus_id, trip_date, departure_time,
			estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			is_bookable, total_seats, available_seats, booked_seats,
			base_fare, booking_advance_hours, assignment_deadline, status, selected_stop_ids
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)
		RETURNING created_at, updated_at
	`

	// Generate ID if not provided
	if trip.ID == "" {
		trip.ID = uuid.New().String()
	}

	err := r.db.QueryRow(
		query,
		trip.ID, trip.TripScheduleID, trip.CustomRouteID, trip.PermitID, trip.BusID, trip.TripDate, trip.DepartureTime,
		trip.EstimatedArrivalTime, trip.AssignedDriverID, trip.AssignedConductorID,
		trip.IsBookable, trip.TotalSeats, trip.AvailableSeats, trip.BookedSeats,
		trip.BaseFare, trip.BookingAdvanceHours, trip.AssignmentDeadline, trip.Status, trip.SelectedStopIDs,
	).Scan(&trip.CreatedAt, &trip.UpdatedAt)

	return err
}

// GetByID retrieves a scheduled trip by ID
func (r *ScheduledTripRepository) GetByID(tripID string) (*models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, permit_id, bus_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, total_seats, available_seats, booked_seats,
			   base_fare, status, cancellation_reason, cancelled_at,
			   selected_stop_ids, created_at, updated_at
		FROM scheduled_trips
		WHERE id = $1
	`

	return r.scanTrip(r.db.QueryRow(query, tripID))
}

// GetByScheduleAndDate checks if a trip exists for a schedule on a specific date
func (r *ScheduledTripRepository) GetByScheduleAndDate(scheduleID string, date time.Time) (*models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, permit_id, bus_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, total_seats, available_seats, booked_seats,
			   base_fare, status, cancellation_reason, cancelled_at,
			   selected_stop_ids, created_at, updated_at
		FROM scheduled_trips
		WHERE trip_schedule_id = $1 AND trip_date = $2
	`

	return r.scanTrip(r.db.QueryRow(query, scheduleID, date))
}

// GetByDateRange retrieves scheduled trips within a date range
func (r *ScheduledTripRepository) GetByDateRange(startDate, endDate time.Time) ([]models.ScheduledTrip, error) {
	query := `
		SELECT id, trip_schedule_id, permit_id, bus_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, total_seats, available_seats, booked_seats,
			   base_fare, status, cancellation_reason, cancelled_at,
			   selected_stop_ids, created_at, updated_at
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
		SELECT id, trip_schedule_id, permit_id, bus_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, total_seats, available_seats, booked_seats,
			   base_fare, status, cancellation_reason, cancelled_at,
			   selected_stop_ids, created_at, updated_at
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
		SELECT id, trip_schedule_id, permit_id, bus_id, trip_date, departure_time,
			   estimated_arrival_time, assigned_driver_id, assigned_conductor_id,
			   is_bookable, total_seats, available_seats, booked_seats,
			   base_fare, status, cancellation_reason, cancelled_at,
			   selected_stop_ids, created_at, updated_at
		FROM scheduled_trips
		WHERE is_bookable = true
		  AND trip_date BETWEEN $1 AND $2
		  AND status IN ('scheduled', 'confirmed')
		  AND available_seats > 0
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
		SET bus_id = $2, assigned_driver_id = $3, assigned_conductor_id = $4,
			total_seats = $5, available_seats = $6, booked_seats = $7,
			status = $8, cancellation_reason = $9, cancelled_at = $10,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		trip.ID, trip.BusID, trip.AssignedDriverID, trip.AssignedConductorID,
		trip.TotalSeats, trip.AvailableSeats, trip.BookedSeats,
		trip.Status, trip.CancellationReason, trip.CancelledAt,
	).Scan(&trip.UpdatedAt)

	return err
}

// UpdateSeats updates the seat counts for a scheduled trip
func (r *ScheduledTripRepository) UpdateSeats(tripID string, bookedSeats, availableSeats int) error {
	query := `
		UPDATE scheduled_trips
		SET booked_seats = $2, available_seats = $3, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(query, tripID, bookedSeats, availableSeats)
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
	var busID sql.NullString
	var estimatedArrivalTime sql.NullString
	var assignedDriverID sql.NullString
	var assignedConductorID sql.NullString
	var cancellationReason sql.NullString
	var cancelledAt sql.NullTime

	err := row.Scan(
		&trip.ID, &trip.TripScheduleID, &trip.PermitID, &busID, &trip.TripDate, &trip.DepartureTime,
		&estimatedArrivalTime, &assignedDriverID, &assignedConductorID,
		&trip.IsBookable, &trip.TotalSeats, &trip.AvailableSeats, &trip.BookedSeats,
		&trip.BaseFare, &trip.Status, &cancellationReason, &cancelledAt,
		&trip.SelectedStopIDs, &trip.CreatedAt, &trip.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if busID.Valid {
		trip.BusID = &busID.String
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
		var busID sql.NullString
		var estimatedArrivalTime sql.NullString
		var assignedDriverID sql.NullString
		var assignedConductorID sql.NullString
		var cancellationReason sql.NullString
		var cancelledAt sql.NullTime

		err := rows.Scan(
			&trip.ID, &trip.TripScheduleID, &trip.PermitID, &busID, &trip.TripDate, &trip.DepartureTime,
			&estimatedArrivalTime, &assignedDriverID, &assignedConductorID,
			&trip.IsBookable, &trip.TotalSeats, &trip.AvailableSeats, &trip.BookedSeats,
			&trip.BaseFare, &trip.Status, &cancellationReason, &cancelledAt,
			&trip.SelectedStopIDs, &trip.CreatedAt, &trip.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if busID.Valid {
			trip.BusID = &busID.String
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
