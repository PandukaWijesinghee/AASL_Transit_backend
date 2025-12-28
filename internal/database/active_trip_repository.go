package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// ActiveTripRepository handles database operations for active_trips table
type ActiveTripRepository struct {
	db DB
}

// NewActiveTripRepository creates a new ActiveTripRepository
func NewActiveTripRepository(db DB) *ActiveTripRepository {
	return &ActiveTripRepository{db: db}
}

// Create creates a new active trip
func (r *ActiveTripRepository) Create(trip *models.ActiveTrip) error {
	query := `
		INSERT INTO active_trips (
			id, scheduled_trip_id, bus_id, permit_id, driver_id, conductor_id,
			status, current_passenger_count, tracking_device_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		RETURNING created_at, updated_at
	`

	// Generate ID if not provided
	if trip.ID == "" {
		trip.ID = uuid.New().String()
	}

	err := r.db.QueryRow(
		query,
		trip.ID, trip.ScheduledTripID, trip.BusID, trip.PermitID, trip.DriverID, trip.ConductorID,
		trip.Status, trip.CurrentPassengerCount, trip.TrackingDeviceID,
	).Scan(&trip.CreatedAt, &trip.UpdatedAt)

	return err
}

// GetByID retrieves an active trip by ID
func (r *ActiveTripRepository) GetByID(tripID string) (*models.ActiveTrip, error) {
	query := `
		SELECT id, scheduled_trip_id, bus_id, permit_id, driver_id, conductor_id,
			   current_latitude, current_longitude, last_location_update,
			   current_speed_kmh, heading, current_stop_id, next_stop_id,
			   stops_completed, actual_departure_time, estimated_arrival_time,
			   actual_arrival_time, status, current_passenger_count,
			   tracking_device_id, created_at, updated_at
		FROM active_trips
		WHERE id = $1
	`

	return r.scanTrip(r.db.QueryRow(query, tripID))
}

// GetByScheduledTripID retrieves an active trip by scheduled trip ID
func (r *ActiveTripRepository) GetByScheduledTripID(scheduledTripID string) (*models.ActiveTrip, error) {
	query := `
		SELECT id, scheduled_trip_id, bus_id, permit_id, driver_id, conductor_id,
			   current_latitude, current_longitude, last_location_update,
			   current_speed_kmh, heading, current_stop_id, next_stop_id,
			   stops_completed, actual_departure_time, estimated_arrival_time,
			   actual_arrival_time, status, current_passenger_count,
			   tracking_device_id, created_at, updated_at
		FROM active_trips
		WHERE scheduled_trip_id = $1
	`

	return r.scanTrip(r.db.QueryRow(query, scheduledTripID))
}

// GetActiveTripsByBusOwner retrieves all active trips for a bus owner
func (r *ActiveTripRepository) GetActiveTripsByBusOwner(busOwnerID string) ([]models.ActiveTrip, error) {
	query := `
		SELECT at.id, at.scheduled_trip_id, at.bus_id, at.permit_id, at.driver_id, at.conductor_id,
			   at.current_latitude, at.current_longitude, at.last_location_update,
			   at.current_speed_kmh, at.heading, at.current_stop_id, at.next_stop_id,
			   at.stops_completed, at.actual_departure_time, at.estimated_arrival_time,
			   at.actual_arrival_time, at.status, at.current_passenger_count,
			   at.tracking_device_id, at.created_at, at.updated_at
		FROM active_trips at
		INNER JOIN route_permits rp ON at.permit_id = rp.id
		WHERE rp.bus_owner_id = $1
		  AND at.status IN ('not_started', 'in_transit', 'at_stop')
		ORDER BY at.created_at DESC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTrips(rows)
}

// GetAllActiveTrips retrieves all currently active trips
func (r *ActiveTripRepository) GetAllActiveTrips() ([]models.ActiveTrip, error) {
	query := `
		SELECT id, scheduled_trip_id, bus_id, permit_id, driver_id, conductor_id,
			   current_latitude, current_longitude, last_location_update,
			   current_speed_kmh, heading, current_stop_id, next_stop_id,
			   stops_completed, actual_departure_time, estimated_arrival_time,
			   actual_arrival_time, status, current_passenger_count,
			   tracking_device_id, created_at, updated_at
		FROM active_trips
		WHERE status IN ('not_started', 'in_transit', 'at_stop')
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTrips(rows)
}

// Update updates an active trip
func (r *ActiveTripRepository) Update(trip *models.ActiveTrip) error {
	query := `
		UPDATE active_trips
		SET current_latitude = $2, current_longitude = $3, last_location_update = $4,
			current_speed_kmh = $5, heading = $6, current_stop_id = $7,
			next_stop_id = $8, stops_completed = $9, actual_departure_time = $10,
			estimated_arrival_time = $11, actual_arrival_time = $12,
			status = $13, current_passenger_count = $14, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		trip.ID, trip.CurrentLatitude, trip.CurrentLongitude, trip.LastLocationUpdate,
		trip.CurrentSpeedKmh, trip.Heading, trip.CurrentStopID,
		trip.NextStopID, trip.StopsCompleted, trip.ActualDepartureTime,
		trip.EstimatedArrivalTime, trip.ActualArrivalTime,
		trip.Status, trip.CurrentPassengerCount,
	).Scan(&trip.UpdatedAt)

	return err
}

// UpdateLocation updates only the location data of an active trip
func (r *ActiveTripRepository) UpdateLocation(tripID string, lat, lng float64, speedKmh, heading *float64) error {
	query := `
		UPDATE active_trips
		SET current_latitude = $2, current_longitude = $3,
			current_speed_kmh = $4, heading = $5,
			last_location_update = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(query, tripID, lat, lng, speedKmh, heading)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("active trip not found")
	}

	return nil
}

// UpdateStatus updates the status of an active trip
func (r *ActiveTripRepository) UpdateStatus(tripID string, status models.ActiveTripStatus) error {
	query := `
		UPDATE active_trips
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
		return fmt.Errorf("active trip not found")
	}

	return nil
}

// GetPassengersByScheduledTripID retrieves passengers by scheduled trip ID
func (r *ActiveTripRepository) GetPassengersByScheduledTripID(scheduledTripID string) ([]models.PassengerInfo, error) {
	var passengers []models.PassengerInfo

	query := `
		SELECT 
			bb.id as booking_id,
			b.booking_reference,
			bb.status,
			bb.boarded_at,
			b.created_at,
			COALESCE(b.passenger_name, '') as passenger_name,
			COALESCE(b.passenger_phone, '') as passenger_phone,
			COALESCE(bor.custom_route_name, mr.route_name, '') as route_name,
			COALESCE(mrs_board.stop_name, '') as pickup_stop_name,
			COALESCE(mrs_alight.stop_name, '') as dropoff_stop_name
		FROM bus_bookings bb
		INNER JOIN bookings b ON bb.booking_id = b.id
		INNER JOIN scheduled_trips st ON bb.scheduled_trip_id = st.id
		LEFT JOIN bus_owner_routes bor ON st.bus_owner_route_id = bor.id
		LEFT JOIN master_routes mr ON bor.master_route_id = mr.id
		LEFT JOIN master_route_stops mrs_board ON bb.boarding_stop_id = mrs_board.id
		LEFT JOIN master_route_stops mrs_alight ON bb.alighting_stop_id = mrs_alight.id
		WHERE bb.scheduled_trip_id = $1
		AND bb.status NOT IN ('cancelled')
		ORDER BY b.created_at DESC
	`

	rows, err := r.db.Query(query, scheduledTripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p models.PassengerInfo
		err := rows.Scan(
			&p.BookingID, &p.BookingReference, &p.Status, &p.BoardedAt, &p.CreatedAt,
			&p.PassengerName, &p.PassengerPhone, &p.RouteName,
			&p.PickupStopName, &p.DropoffStopName,
		)
		if err != nil {
			return nil, err
		}
		passengers = append(passengers, p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return passengers, nil
}

// scanTrip scans a single active trip
func (r *ActiveTripRepository) scanTrip(row scanner) (*models.ActiveTrip, error) {
	trip := &models.ActiveTrip{}
	var conductorID sql.NullString
	var currentLatitude sql.NullFloat64
	var currentLongitude sql.NullFloat64
	var lastLocationUpdate sql.NullTime
	var currentSpeedKmh sql.NullFloat64
	var heading sql.NullFloat64
	var currentStopID sql.NullString
	var nextStopID sql.NullString
	var actualDepartureTime sql.NullTime
	var estimatedArrivalTime sql.NullTime
	var actualArrivalTime sql.NullTime
	var trackingDeviceID sql.NullString

	err := row.Scan(
		&trip.ID, &trip.ScheduledTripID, &trip.BusID, &trip.PermitID, &trip.DriverID, &conductorID,
		&currentLatitude, &currentLongitude, &lastLocationUpdate,
		&currentSpeedKmh, &heading, &currentStopID, &nextStopID,
		&trip.StopsCompleted, &actualDepartureTime, &estimatedArrivalTime,
		&actualArrivalTime, &trip.Status, &trip.CurrentPassengerCount,
		&trackingDeviceID, &trip.CreatedAt, &trip.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if conductorID.Valid {
		trip.ConductorID = &conductorID.String
	}
	if currentLatitude.Valid {
		trip.CurrentLatitude = &currentLatitude.Float64
	}
	if currentLongitude.Valid {
		trip.CurrentLongitude = &currentLongitude.Float64
	}
	if lastLocationUpdate.Valid {
		trip.LastLocationUpdate = &lastLocationUpdate.Time
	}
	if currentSpeedKmh.Valid {
		trip.CurrentSpeedKmh = &currentSpeedKmh.Float64
	}
	if heading.Valid {
		trip.Heading = &heading.Float64
	}
	if currentStopID.Valid {
		trip.CurrentStopID = &currentStopID.String
	}
	if nextStopID.Valid {
		trip.NextStopID = &nextStopID.String
	}
	if actualDepartureTime.Valid {
		trip.ActualDepartureTime = &actualDepartureTime.Time
	}
	if estimatedArrivalTime.Valid {
		trip.EstimatedArrivalTime = &estimatedArrivalTime.Time
	}
	if actualArrivalTime.Valid {
		trip.ActualArrivalTime = &actualArrivalTime.Time
	}
	if trackingDeviceID.Valid {
		trip.TrackingDeviceID = &trackingDeviceID.String
	}

	return trip, nil
}

// scanTrips scans multiple active trips from rows
func (r *ActiveTripRepository) scanTrips(rows *sql.Rows) ([]models.ActiveTrip, error) {
	trips := []models.ActiveTrip{}

	for rows.Next() {
		var trip models.ActiveTrip
		var conductorID sql.NullString
		var currentLatitude sql.NullFloat64
		var currentLongitude sql.NullFloat64
		var lastLocationUpdate sql.NullTime
		var currentSpeedKmh sql.NullFloat64
		var heading sql.NullFloat64
		var currentStopID sql.NullString
		var nextStopID sql.NullString
		var actualDepartureTime sql.NullTime
		var estimatedArrivalTime sql.NullTime
		var actualArrivalTime sql.NullTime
		var trackingDeviceID sql.NullString

		err := rows.Scan(
			&trip.ID, &trip.ScheduledTripID, &trip.BusID, &trip.PermitID, &trip.DriverID, &conductorID,
			&currentLatitude, &currentLongitude, &lastLocationUpdate,
			&currentSpeedKmh, &heading, &currentStopID, &nextStopID,
			&trip.StopsCompleted, &actualDepartureTime, &estimatedArrivalTime,
			&actualArrivalTime, &trip.Status, &trip.CurrentPassengerCount,
			&trackingDeviceID, &trip.CreatedAt, &trip.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if conductorID.Valid {
			trip.ConductorID = &conductorID.String
		}
		if currentLatitude.Valid {
			trip.CurrentLatitude = &currentLatitude.Float64
		}
		if currentLongitude.Valid {
			trip.CurrentLongitude = &currentLongitude.Float64
		}
		if lastLocationUpdate.Valid {
			trip.LastLocationUpdate = &lastLocationUpdate.Time
		}
		if currentSpeedKmh.Valid {
			trip.CurrentSpeedKmh = &currentSpeedKmh.Float64
		}
		if heading.Valid {
			trip.Heading = &heading.Float64
		}
		if currentStopID.Valid {
			trip.CurrentStopID = &currentStopID.String
		}
		if nextStopID.Valid {
			trip.NextStopID = &nextStopID.String
		}
		if actualDepartureTime.Valid {
			trip.ActualDepartureTime = &actualDepartureTime.Time
		}
		if estimatedArrivalTime.Valid {
			trip.EstimatedArrivalTime = &estimatedArrivalTime.Time
		}
		if actualArrivalTime.Valid {
			trip.ActualArrivalTime = &actualArrivalTime.Time
		}
		if trackingDeviceID.Valid {
			trip.TrackingDeviceID = &trackingDeviceID.String
		}

		trips = append(trips, trip)
	}

	return trips, rows.Err()
}
