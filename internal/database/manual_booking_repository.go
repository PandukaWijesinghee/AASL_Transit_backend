package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// ManualBookingRepository handles manual_seat_bookings database operations
type ManualBookingRepository struct {
	db *sqlx.DB
}

// NewManualBookingRepository creates a new ManualBookingRepository
func NewManualBookingRepository(db *sqlx.DB) *ManualBookingRepository {
	return &ManualBookingRepository{db: db}
}

// GetNextSequenceNumber returns the next sequence number for booking reference
func (r *ManualBookingRepository) GetNextSequenceNumber(bookingType models.ManualBookingType) (int, error) {
	prefix := "MB"
	switch bookingType {
	case models.ManualBookingTypePhone:
		prefix = "PH"
	case models.ManualBookingTypeAgent:
		prefix = "AG"
	case models.ManualBookingTypeWalkIn:
		prefix = "WI"
	}

	datePart := time.Now().Format("20060102")
	pattern := prefix + "-" + datePart + "-%"

	query := `
		SELECT COALESCE(MAX(
			CAST(SUBSTRING(booking_reference FROM '[0-9]+$') AS integer)
		), 0) + 1
		FROM manual_seat_bookings
		WHERE booking_reference LIKE $1
	`

	var seq int
	err := r.db.Get(&seq, query, pattern)
	if err != nil {
		return 1, nil // Start from 1 if no bookings exist
	}

	return seq, nil
}

// Create creates a new manual booking and its seats in a transaction
func (r *ManualBookingRepository) Create(booking *models.ManualSeatBooking, seatIDs []string, tripSeatRepo *TripSeatRepository) (*models.ManualBookingWithSeats, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Get seat details to calculate total fare
	seats, err := tripSeatRepo.GetByIDs(seatIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get seat details: %w", err)
	}

	if len(seats) != len(seatIDs) {
		return nil, fmt.Errorf("some seats not found")
	}

	// Check all seats are available
	for _, seat := range seats {
		if seat.Status != models.TripSeatStatusAvailable {
			return nil, fmt.Errorf("seat %s is not available (status: %s)", seat.SeatNumber, seat.Status)
		}
	}

	// Calculate total fare
	var totalFare float64
	for _, seat := range seats {
		totalFare += seat.SeatPrice
	}

	// 2. Generate booking reference
	seq, _ := r.GetNextSequenceNumber(booking.BookingType)
	booking.BookingReference = models.GenerateBookingReference(booking.BookingType, seq)
	booking.NumberOfSeats = len(seatIDs)
	booking.TotalFare = totalFare

	// 3. Insert the manual booking
	insertBookingQuery := `
		INSERT INTO manual_seat_bookings (
			booking_reference, scheduled_trip_id, created_by_user_id, booking_type,
			passenger_name, passenger_phone, passenger_nic, passenger_notes,
			route_name, boarding_stop_id, boarding_stop_name, alighting_stop_id, alighting_stop_name,
			departure_datetime, number_of_seats, total_fare,
			payment_status, amount_paid, payment_method, payment_notes,
			status, confirmed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		) RETURNING id, created_at, updated_at
	`

	now := time.Now()
	err = tx.QueryRowx(insertBookingQuery,
		booking.BookingReference,
		booking.ScheduledTripID,
		booking.CreatedByUserID,
		booking.BookingType,
		booking.PassengerName,
		booking.PassengerPhone,
		booking.PassengerNIC,
		booking.PassengerNotes,
		booking.RouteName,
		booking.BoardingStopID,
		booking.BoardingStopName,
		booking.AlightingStopID,
		booking.AlightingStopName,
		booking.DepartureDatetime,
		booking.NumberOfSeats,
		booking.TotalFare,
		booking.PaymentStatus,
		booking.AmountPaid,
		booking.PaymentMethod,
		booking.PaymentNotes,
		models.ManualBookingStatusConfirmed,
		now,
	).Scan(&booking.ID, &booking.CreatedAt, &booking.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert manual booking: %w", err)
	}

	booking.Status = models.ManualBookingStatusConfirmed
	booking.ConfirmedAt = &now

	// 4. Insert manual_booking_seats and update trip_seats
	var bookingSeats []models.ManualBookingSeat
	bookingType := models.TripSeatBookingTypePhone
	switch booking.BookingType {
	case models.ManualBookingTypePhone:
		bookingType = models.TripSeatBookingTypePhone
	case models.ManualBookingTypeAgent:
		bookingType = models.TripSeatBookingTypeAgent
	case models.ManualBookingTypeWalkIn:
		bookingType = models.TripSeatBookingTypeWalkIn
	}

	for _, seat := range seats {
		// Insert into manual_booking_seats
		insertSeatQuery := `
			INSERT INTO manual_booking_seats (manual_booking_id, trip_seat_id, seat_number, seat_price)
			VALUES ($1, $2, $3, $4)
			RETURNING id, created_at
		`
		var bookingSeat models.ManualBookingSeat
		bookingSeat.ManualBookingID = booking.ID
		bookingSeat.TripSeatID = seat.ID
		bookingSeat.SeatNumber = seat.SeatNumber
		bookingSeat.SeatPrice = seat.SeatPrice

		err = tx.QueryRowx(insertSeatQuery, booking.ID, seat.ID, seat.SeatNumber, seat.SeatPrice).
			Scan(&bookingSeat.ID, &bookingSeat.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to insert booking seat: %w", err)
		}

		bookingSeats = append(bookingSeats, bookingSeat)

		// Update trip_seat status
		updateSeatQuery := `
			UPDATE trip_seats
			SET status = 'booked',
				booking_type = $1,
				manual_booking_id = $2,
				updated_at = $3
			WHERE id = $4 AND status = 'available'
		`
		result, err := tx.Exec(updateSeatQuery, string(bookingType), booking.ID, now, seat.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update trip seat: %w", err)
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return nil, fmt.Errorf("seat %s is no longer available", seat.SeatNumber)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.ManualBookingWithSeats{
		ManualSeatBooking: *booking,
		Seats:             bookingSeats,
	}, nil
}

// GetByID returns a manual booking by ID
func (r *ManualBookingRepository) GetByID(id string) (*models.ManualSeatBooking, error) {
	query := `
		SELECT id, booking_reference, scheduled_trip_id, created_by_user_id, booking_type,
			   passenger_name, passenger_phone, passenger_nic, passenger_notes,
			   route_name, boarding_stop_id, boarding_stop_name, alighting_stop_id, alighting_stop_name,
			   departure_datetime, number_of_seats, total_fare,
			   payment_status, amount_paid, payment_method, payment_notes,
			   status, confirmed_at, checked_in_at, boarded_at, completed_at,
			   cancelled_at, cancellation_reason, created_at, updated_at
		FROM manual_seat_bookings
		WHERE id = $1
	`

	var booking models.ManualSeatBooking
	err := r.db.Get(&booking, query, id)
	if err != nil {
		return nil, err
	}

	return &booking, nil
}

// GetByBookingReference returns a manual booking by reference
func (r *ManualBookingRepository) GetByBookingReference(ref string) (*models.ManualSeatBooking, error) {
	query := `
		SELECT id, booking_reference, scheduled_trip_id, created_by_user_id, booking_type,
			   passenger_name, passenger_phone, passenger_nic, passenger_notes,
			   route_name, boarding_stop_id, boarding_stop_name, alighting_stop_id, alighting_stop_name,
			   departure_datetime, number_of_seats, total_fare,
			   payment_status, amount_paid, payment_method, payment_notes,
			   status, confirmed_at, checked_in_at, boarded_at, completed_at,
			   cancelled_at, cancellation_reason, created_at, updated_at
		FROM manual_seat_bookings
		WHERE booking_reference = $1
	`

	var booking models.ManualSeatBooking
	err := r.db.Get(&booking, query, ref)
	if err != nil {
		return nil, err
	}

	return &booking, nil
}

// GetByScheduledTripID returns all manual bookings for a trip
func (r *ManualBookingRepository) GetByScheduledTripID(scheduledTripID string) ([]models.ManualSeatBooking, error) {
	query := `
		SELECT id, booking_reference, scheduled_trip_id, created_by_user_id, booking_type,
			   passenger_name, passenger_phone, passenger_nic, passenger_notes,
			   route_name, boarding_stop_id, boarding_stop_name, alighting_stop_id, alighting_stop_name,
			   departure_datetime, number_of_seats, total_fare,
			   payment_status, amount_paid, payment_method, payment_notes,
			   status, confirmed_at, checked_in_at, boarded_at, completed_at,
			   cancelled_at, cancellation_reason, created_at, updated_at
		FROM manual_seat_bookings
		WHERE scheduled_trip_id = $1
		ORDER BY created_at DESC
	`

	var bookings []models.ManualSeatBooking
	err := r.db.Select(&bookings, query, scheduledTripID)
	if err != nil {
		return nil, err
	}

	return bookings, nil
}

// GetBookingSeats returns all seats for a manual booking
func (r *ManualBookingRepository) GetBookingSeats(manualBookingID string) ([]models.ManualBookingSeat, error) {
	query := `
		SELECT id, manual_booking_id, trip_seat_id, seat_number, seat_price, passenger_name, created_at
		FROM manual_booking_seats
		WHERE manual_booking_id = $1
		ORDER BY seat_number
	`

	var seats []models.ManualBookingSeat
	err := r.db.Select(&seats, query, manualBookingID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// GetWithSeats returns a manual booking with its seats
func (r *ManualBookingRepository) GetWithSeats(id string) (*models.ManualBookingWithSeats, error) {
	booking, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	seats, err := r.GetBookingSeats(id)
	if err != nil {
		return nil, err
	}

	return &models.ManualBookingWithSeats{
		ManualSeatBooking: *booking,
		Seats:             seats,
	}, nil
}

// UpdatePayment updates payment information
func (r *ManualBookingRepository) UpdatePayment(id string, paymentStatus models.ManualBookingPaymentStatus, amountPaid float64, paymentMethod, paymentNotes *string) error {
	query := `
		UPDATE manual_seat_bookings
		SET payment_status = $1,
			amount_paid = $2,
			payment_method = $3,
			payment_notes = $4,
			updated_at = $5
		WHERE id = $6
	`

	_, err := r.db.Exec(query, paymentStatus, amountPaid, paymentMethod, paymentNotes, time.Now(), id)
	return err
}

// UpdateStatus updates booking status
func (r *ManualBookingRepository) UpdateStatus(id string, status models.ManualBookingStatus) error {
	now := time.Now()
	var query string

	switch status {
	case models.ManualBookingStatusCheckedIn:
		query = `UPDATE manual_seat_bookings SET status = $1, checked_in_at = $2, updated_at = $2 WHERE id = $3`
	case models.ManualBookingStatusBoarded:
		query = `UPDATE manual_seat_bookings SET status = $1, boarded_at = $2, updated_at = $2 WHERE id = $3`
	case models.ManualBookingStatusCompleted:
		query = `UPDATE manual_seat_bookings SET status = $1, completed_at = $2, updated_at = $2 WHERE id = $3`
	case models.ManualBookingStatusNoShow:
		query = `UPDATE manual_seat_bookings SET status = $1, updated_at = $2 WHERE id = $3`
	default:
		query = `UPDATE manual_seat_bookings SET status = $1, updated_at = $2 WHERE id = $3`
	}

	_, err := r.db.Exec(query, status, now, id)
	return err
}

// Cancel cancels a manual booking and releases the seats
func (r *ManualBookingRepository) Cancel(id, reason string, tripSeatRepo *TripSeatRepository) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Update booking status
	updateBookingQuery := `
		UPDATE manual_seat_bookings
		SET status = 'cancelled',
			cancelled_at = $1,
			cancellation_reason = $2,
			updated_at = $1
		WHERE id = $3 AND status NOT IN ('cancelled', 'completed')
	`

	result, err := tx.Exec(updateBookingQuery, now, reason, id)
	if err != nil {
		return fmt.Errorf("failed to cancel booking: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	// Release the seats
	releaseSeatQuery := `
		UPDATE trip_seats
		SET status = 'available',
			booking_type = NULL,
			manual_booking_id = NULL,
			updated_at = $1
		WHERE manual_booking_id = $2
	`

	_, err = tx.Exec(releaseSeatQuery, now, id)
	if err != nil {
		return fmt.Errorf("failed to release seats: %w", err)
	}

	return tx.Commit()
}

// GetByCreatorUserID returns all manual bookings created by a user
func (r *ManualBookingRepository) GetByCreatorUserID(userID string, limit, offset int) ([]models.ManualSeatBooking, error) {
	query := `
		SELECT id, booking_reference, scheduled_trip_id, created_by_user_id, booking_type,
			   passenger_name, passenger_phone, passenger_nic, passenger_notes,
			   route_name, boarding_stop_id, boarding_stop_name, alighting_stop_id, alighting_stop_name,
			   departure_datetime, number_of_seats, total_fare,
			   payment_status, amount_paid, payment_method, payment_notes,
			   status, confirmed_at, checked_in_at, boarded_at, completed_at,
			   cancelled_at, cancellation_reason, created_at, updated_at
		FROM manual_seat_bookings
		WHERE created_by_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	var bookings []models.ManualSeatBooking
	err := r.db.Select(&bookings, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	return bookings, nil
}

// SearchByPassengerPhone searches bookings by passenger phone number
func (r *ManualBookingRepository) SearchByPassengerPhone(phone string) ([]models.ManualSeatBooking, error) {
	query := `
		SELECT id, booking_reference, scheduled_trip_id, created_by_user_id, booking_type,
			   passenger_name, passenger_phone, passenger_nic, passenger_notes,
			   route_name, boarding_stop_id, boarding_stop_name, alighting_stop_id, alighting_stop_name,
			   departure_datetime, number_of_seats, total_fare,
			   payment_status, amount_paid, payment_method, payment_notes,
			   status, confirmed_at, checked_in_at, boarded_at, completed_at,
			   cancelled_at, cancellation_reason, created_at, updated_at
		FROM manual_seat_bookings
		WHERE passenger_phone LIKE $1
		ORDER BY created_at DESC
		LIMIT 50
	`

	var bookings []models.ManualSeatBooking
	err := r.db.Select(&bookings, query, "%"+phone+"%")
	if err != nil {
		return nil, err
	}

	return bookings, nil
}
