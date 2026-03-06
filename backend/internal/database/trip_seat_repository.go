package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// TripSeatRepository handles trip_seats database operations
type TripSeatRepository struct {
	db *sqlx.DB
}

// NewTripSeatRepository creates a new TripSeatRepository
func NewTripSeatRepository(db *sqlx.DB) *TripSeatRepository {
	return &TripSeatRepository{db: db}
}

// CreateTripSeatsFromLayout creates trip_seats from a seat layout template
// This is called when assigning a seat layout to a scheduled trip
func (r *TripSeatRepository) CreateTripSeatsFromLayout(scheduledTripID, seatLayoutID string, baseFare float64) (int, error) {
	// First, delete any existing trip seats for this trip
	_, err := r.db.Exec(`DELETE FROM trip_seats WHERE scheduled_trip_id = $1`, scheduledTripID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete existing trip seats: %w", err)
	}

	// Get seats from the layout template
	query := `
		SELECT 
			seat_number,
			row_number,
			position,
			CASE 
				WHEN is_window_seat THEN 'window'
				WHEN is_aisle_seat THEN 'aisle'
				ELSE 'standard'
			END as seat_type
		FROM bus_seat_layout_seats
		WHERE template_id = $1
		ORDER BY row_number, position
	`

	type layoutSeat struct {
		SeatNumber string `db:"seat_number"`
		RowNumber  int    `db:"row_number"`
		Position   int    `db:"position"`
		SeatType   string `db:"seat_type"`
	}

	var seats []layoutSeat
	err = r.db.Select(&seats, query, seatLayoutID)
	if err != nil {
		return 0, fmt.Errorf("failed to get layout seats: %w", err)
	}

	if len(seats) == 0 {
		return 0, fmt.Errorf("no seats found in layout template")
	}

	// Insert trip seats
	insertQuery := `
		INSERT INTO trip_seats (
			scheduled_trip_id, seat_number, seat_type, row_number, position,
			seat_price, status, booking_type
		) VALUES ($1, $2, $3, $4, $5, $6, 'available', NULL)
	`

	count := 0
	for _, seat := range seats {
		_, err := r.db.Exec(insertQuery,
			scheduledTripID,
			seat.SeatNumber,
			seat.SeatType,
			seat.RowNumber,
			seat.Position,
			baseFare,
		)
		if err != nil {
			return count, fmt.Errorf("failed to insert trip seat %s: %w", seat.SeatNumber, err)
		}
		count++
	}

	return count, nil
}

// GetByScheduledTripID returns all seats for a scheduled trip
func (r *TripSeatRepository) GetByScheduledTripID(scheduledTripID string) ([]models.TripSeat, error) {
	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
			   block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE scheduled_trip_id = $1
		ORDER BY row_number, position
	`

	var seats []models.TripSeat
	err := r.db.Select(&seats, query, scheduledTripID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// GetByScheduledTripIDWithBookingInfo returns seats with booking details
func (r *TripSeatRepository) GetByScheduledTripIDWithBookingInfo(scheduledTripID string) ([]models.TripSeatWithBookingInfo, error) {
	query := `
		SELECT 
			ts.id, ts.scheduled_trip_id, ts.seat_number, ts.seat_type, ts.row_number, ts.position,
			ts.seat_price, ts.status, ts.booking_type, ts.bus_booking_seat_id, ts.manual_booking_id,
			ts.block_reason, ts.blocked_by_user_id, ts.blocked_at, ts.created_at, ts.updated_at,
			mb.passenger_name, mb.passenger_phone, mb.booking_reference, mb.payment_status
		FROM trip_seats ts
		LEFT JOIN manual_seat_bookings mb ON ts.manual_booking_id = mb.id
		WHERE ts.scheduled_trip_id = $1
		ORDER BY ts.row_number, ts.position
	`

	var seats []models.TripSeatWithBookingInfo
	err := r.db.Select(&seats, query, scheduledTripID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// GetByID returns a single trip seat by ID
func (r *TripSeatRepository) GetByID(id string) (*models.TripSeat, error) {
	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
			   block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE id = $1
	`

	var seat models.TripSeat
	err := r.db.Get(&seat, query, id)
	if err != nil {
		return nil, err
	}

	return &seat, nil
}

// GetByIDs returns multiple trip seats by IDs
func (r *TripSeatRepository) GetByIDs(ids []string) ([]models.TripSeat, error) {
	if len(ids) == 0 {
		return []models.TripSeat{}, nil
	}

	query, args, err := sqlx.In(`
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
			   block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE id IN (?)
		ORDER BY row_number, position
	`, ids)
	if err != nil {
		return nil, err
	}

	query = r.db.Rebind(query)

	var seats []models.TripSeat
	err = r.db.Select(&seats, query, args...)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// GetSummary returns seat availability summary for a trip
func (r *TripSeatRepository) GetSummary(scheduledTripID string) (*models.TripSeatSummary, error) {
	query := `
		SELECT 
			scheduled_trip_id,
			COUNT(*) as total_seats,
			COUNT(*) FILTER (WHERE status = 'available') as available_seats,
			COUNT(*) FILTER (WHERE status = 'booked') as booked_seats,
			COUNT(*) FILTER (WHERE status = 'blocked') as blocked_seats,
			COUNT(*) FILTER (WHERE status = 'reserved') as reserved_seats,
			COUNT(*) FILTER (WHERE booking_type = 'app') as app_bookings,
			COUNT(*) FILTER (WHERE booking_type = 'phone') as phone_bookings,
			COUNT(*) FILTER (WHERE booking_type = 'agent') as agent_bookings,
			COUNT(*) FILTER (WHERE booking_type = 'walk_in') as walk_in_bookings
		FROM trip_seats
		WHERE scheduled_trip_id = $1
		GROUP BY scheduled_trip_id
	`

	var summary models.TripSeatSummary
	err := r.db.Get(&summary, query, scheduledTripID)
	if err != nil {
		if err == sql.ErrNoRows {
			// No seats exist yet
			return &models.TripSeatSummary{ScheduledTripID: scheduledTripID}, nil
		}
		return nil, err
	}

	return &summary, nil
}

// BlockSeats blocks one or more seats
func (r *TripSeatRepository) BlockSeats(seatIDs []string, blockedByUserID, reason string) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET status = 'blocked',
			booking_type = 'blocked',
			block_reason = ?,
			blocked_by_user_id = ?,
			blocked_at = ?,
			updated_at = ?
		WHERE id IN (?) AND status = 'available'
	`, reason, blockedByUserID, time.Now(), time.Now(), seatIDs)
	if err != nil {
		return 0, err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// UnblockSeats unblocks one or more seats
func (r *TripSeatRepository) UnblockSeats(seatIDs []string) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET status = 'available',
			booking_type = NULL,
			block_reason = NULL,
			blocked_by_user_id = NULL,
			blocked_at = NULL,
			updated_at = ?
		WHERE id IN (?) AND status = 'blocked'
	`, time.Now(), seatIDs)
	if err != nil {
		return 0, err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// UpdateSeatPrices updates the price for multiple seats
func (r *TripSeatRepository) UpdateSeatPrices(seatIDs []string, newPrice float64) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET seat_price = ?,
			updated_at = ?
		WHERE id IN (?)
	`, newPrice, time.Now(), seatIDs)
	if err != nil {
		return 0, err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// BookSeatsForManualBooking marks seats as booked for a manual booking
func (r *TripSeatRepository) BookSeatsForManualBooking(seatIDs []string, manualBookingID string, bookingType models.TripSeatBookingType) error {
	if len(seatIDs) == 0 {
		return nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET status = 'booked',
			booking_type = ?,
			manual_booking_id = ?,
			updated_at = ?
		WHERE id IN (?) AND status = 'available'
	`, string(bookingType), manualBookingID, time.Now(), seatIDs)
	if err != nil {
		return err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if int(rowsAffected) != len(seatIDs) {
		return fmt.Errorf("some seats are not available, expected %d, updated %d", len(seatIDs), rowsAffected)
	}

	return nil
}

// ReleaseSeatsFromManualBooking releases seats when a manual booking is cancelled
func (r *TripSeatRepository) ReleaseSeatsFromManualBooking(manualBookingID string) error {
	query := `
		UPDATE trip_seats
		SET status = 'available',
			booking_type = NULL,
			manual_booking_id = NULL,
			updated_at = $1
		WHERE manual_booking_id = $2
	`

	_, err := r.db.Exec(query, time.Now(), manualBookingID)
	return err
}

// CheckSeatsAvailable checks if all specified seats are available
func (r *TripSeatRepository) CheckSeatsAvailable(seatIDs []string) (bool, error) {
	if len(seatIDs) == 0 {
		return true, nil
	}

	query, args, err := sqlx.In(`
		SELECT COUNT(*) FROM trip_seats
		WHERE id IN (?) AND status = 'available'
	`, seatIDs)
	if err != nil {
		return false, err
	}

	query = r.db.Rebind(query)

	var count int
	err = r.db.Get(&count, query, args...)
	if err != nil {
		return false, err
	}

	return count == len(seatIDs), nil
}

// DeleteByScheduledTripID deletes all trip seats for a scheduled trip
func (r *TripSeatRepository) DeleteByScheduledTripID(scheduledTripID string) error {
	_, err := r.db.Exec(`DELETE FROM trip_seats WHERE scheduled_trip_id = $1`, scheduledTripID)
	return err
}

// GetAvailableSeats returns only available seats for a trip
func (r *TripSeatRepository) GetAvailableSeats(scheduledTripID string) ([]models.TripSeat, error) {
	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, created_at, updated_at
		FROM trip_seats
		WHERE scheduled_trip_id = $1 AND status = 'available'
		ORDER BY row_number, position
	`

	var seats []models.TripSeat
	err := r.db.Select(&seats, query, scheduledTripID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}
