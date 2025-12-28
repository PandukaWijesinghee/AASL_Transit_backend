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

// GetActiveTripPassengers retrieves all passengers for an active scheduled trip
func (r *ActiveTripRepository) GetActiveTripPassengers(scheduledTripID uint) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			bb.id as booking_id,
			bb.reference_number,
			bb.status,
			bb.boarded_at,
			bb.created_at as booking_time,
			bbs.passenger_name,
			bbs.passenger_phone,
			bbs.seat_number,
			st.id as scheduled_trip_id,
			st.departure_time,
			mr.origin || ' → ' || mr.destination as route_name,
			mr.origin,
			mr.destination
		FROM bus_bookings bb
		INNER JOIN bus_booking_seats bbs ON bb.id = bbs.booking_id
		INNER JOIN scheduled_trips st ON bb.scheduled_trip_id = st.id
		INNER JOIN bus_owner_routes bor ON st.route_id = bor.id
		INNER JOIN master_routes mr ON bor.master_route_id = mr.id
		WHERE bb.scheduled_trip_id = $1
		AND bb.status NOT IN ('cancelled', 'refunded')
		ORDER BY bbs.seat_number ASC, bb.created_at ASC
	`

	rows, err := r.db.Query(query, scheduledTripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var bookingID, scheduledTripID int
		var referenceNumber, status, passengerName, passengerPhone, seatNumber, routeName, origin, destination string
		var boardedAt, bookingTime, departureTime sql.NullTime

		err := rows.Scan(
			&bookingID, &referenceNumber, &status, &boardedAt, &bookingTime,
			&passengerName, &passengerPhone, &seatNumber, &scheduledTripID,
			&departureTime, &routeName, &origin, &destination,
		)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"booking_id":        bookingID,
			"reference_number":  referenceNumber,
			"status":            status,
			"passenger_name":    passengerName,
			"passenger_phone":   passengerPhone,
			"seat_number":       seatNumber,
			"scheduled_trip_id": scheduledTripID,
			"route_name":        routeName,
			"origin":            origin,
			"destination":       destination,
		}

		if boardedAt.Valid {
			result["boarded_at"] = boardedAt.Time
		}
		if bookingTime.Valid {
			result["booking_time"] = bookingTime.Time
		}
		if departureTime.Valid {
			result["departure_time"] = departureTime.Time
		}

		results = append(results, result)
	}

	return results, rows.Err()
}

// UpdatePassengerBoardingStatus updates a passenger's status to boarded
func (r *ActiveTripRepository) UpdatePassengerBoardingStatus(bookingID uint) error {
	query := `
		UPDATE bus_bookings 
		SET status = 'boarded', boarded_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	result, err := r.db.Exec(query, bookingID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("booking not found")
	}

	return nil
}

// UpdatePassengerStatus updates a passenger's booking status
func (r *ActiveTripRepository) UpdatePassengerStatus(bookingID uint, status string) error {
	query := `
		UPDATE bus_bookings 
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	result, err := r.db.Exec(query, status, bookingID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("booking not found")
	}

	return nil
}

// GetBookingByID retrieves a booking by ID with trip and route information
func (r *ActiveTripRepository) GetBookingByID(bookingID uint) (map[string]interface{}, error) {
	query := `
		SELECT 
			bb.id as booking_id,
			bb.reference_number,
			bb.status,
			bb.boarded_at,
			bb.scheduled_trip_id,
			bbs.passenger_name,
			bbs.passenger_phone,
			bbs.seat_number,
			st.status as trip_status,
			mr.origin || ' → ' || mr.destination as route_name
		FROM bus_bookings bb
		INNER JOIN bus_booking_seats bbs ON bb.id = bbs.booking_id
		INNER JOIN scheduled_trips st ON bb.scheduled_trip_id = st.id
		INNER JOIN bus_owner_routes bor ON st.route_id = bor.id
		INNER JOIN master_routes mr ON bor.master_route_id = mr.id
		WHERE bb.id = $1
		LIMIT 1
	`

	var bookingIDRes, scheduledTripID int
	var referenceNumber, status, passengerName, passengerPhone, seatNumber, tripStatus, routeName string
	var boardedAt sql.NullTime

	err := r.db.QueryRow(query, bookingID).Scan(
		&bookingIDRes, &referenceNumber, &status, &boardedAt, &scheduledTripID,
		&passengerName, &passengerPhone, &seatNumber, &tripStatus, &routeName,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"booking_id":        bookingIDRes,
		"reference_number":  referenceNumber,
		"status":            status,
		"scheduled_trip_id": scheduledTripID,
		"passenger_name":    passengerName,
		"passenger_phone":   passengerPhone,
		"seat_number":       seatNumber,
		"trip_status":       tripStatus,
		"route_name":        routeName,
	}

	if boardedAt.Valid {
		result["boarded_at"] = boardedAt.Time
	}

	return result, nil
}

// GetBookingByReference retrieves a booking by reference number for QR verification
func (r *ActiveTripRepository) GetBookingByReference(reference string) (map[string]interface{}, error) {
	query := `
		SELECT 
			bb.id as booking_id,
			bb.reference_number,
			bb.status,
			bb.boarded_at,
			bb.scheduled_trip_id,
			bbs.passenger_name,
			bbs.passenger_phone,
			bbs.seat_number,
			st.status as trip_status,
			mr.origin || ' → ' || mr.destination as route_name,
			at.id as active_trip_id
		FROM bus_bookings bb
		INNER JOIN bus_booking_seats bbs ON bb.id = bbs.booking_id
		INNER JOIN scheduled_trips st ON bb.scheduled_trip_id = st.id
		INNER JOIN bus_owner_routes bor ON st.route_id = bor.id
		INNER JOIN master_routes mr ON bor.master_route_id = mr.id
		LEFT JOIN active_trips at ON st.id = at.scheduled_trip_id AND at.status = 'in_transit'
		WHERE bb.reference_number = $1
		LIMIT 1
	`

	var bookingID, scheduledTripID int
	var referenceNumber, status, passengerName, passengerPhone, seatNumber, tripStatus, routeName string
	var boardedAt sql.NullTime
	var activeTripID sql.NullString

	err := r.db.QueryRow(query, reference).Scan(
		&bookingID, &referenceNumber, &status, &boardedAt, &scheduledTripID,
		&passengerName, &passengerPhone, &seatNumber, &tripStatus, &routeName, &activeTripID,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"booking_id":        bookingID,
		"reference_number":  referenceNumber,
		"status":            status,
		"scheduled_trip_id": scheduledTripID,
		"passenger_name":    passengerName,
		"passenger_phone":   passengerPhone,
		"seat_number":       seatNumber,
		"trip_status":       tripStatus,
		"route_name":        routeName,
	}

	if boardedAt.Valid {
		result["boarded_at"] = boardedAt.Time
	}
	if activeTripID.Valid {
		result["active_trip_id"] = activeTripID.String
	}

	return result, nil
}

// ValidateBookingForActiveTrip checks if a booking belongs to an active scheduled trip
func (r *ActiveTripRepository) ValidateBookingForActiveTrip(bookingID uint) (bool, error) {
	var count int

	query := `
		SELECT COUNT(*) 
		FROM bus_bookings bb
		INNER JOIN scheduled_trips st ON bb.scheduled_trip_id = st.id
		INNER JOIN active_trips at ON st.id = at.scheduled_trip_id
		WHERE bb.id = $1 AND at.status IN ('in_transit', 'at_stop')
	`

	err := r.db.QueryRow(query, bookingID).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
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
