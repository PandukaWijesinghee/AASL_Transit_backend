package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BookingRepository handles database operations for bookings table
type BookingRepository struct {
	db DB
}

// NewBookingRepository creates a new BookingRepository
func NewBookingRepository(db DB) *BookingRepository {
	return &BookingRepository{db: db}
}

// Create creates a new booking
func (r *BookingRepository) Create(booking *models.Booking) error {
	query := `
		INSERT INTO bookings (
			id, scheduled_trip_id, user_id, booking_reference,
			number_of_seats, boarding_stop_id, alighting_stop_id,
			total_fare, payment_status, payment_method, payment_reference,
			booking_status, passenger_name, passenger_phone,
			passenger_email, special_requests
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
		RETURNING created_at, updated_at
	`

	// Generate ID if not provided
	if booking.ID == "" {
		booking.ID = uuid.New().String()
	}

	err := r.db.QueryRow(
		query,
		booking.ID, booking.ScheduledTripID, booking.UserID, booking.BookingReference,
		booking.NumberOfSeats, booking.BoardingStopID, booking.AlightingStopID,
		booking.TotalFare, booking.PaymentStatus, booking.PaymentMethod, booking.PaymentReference,
		booking.BookingStatus, booking.PassengerName, booking.PassengerPhone,
		booking.PassengerEmail, booking.SpecialRequests,
	).Scan(&booking.CreatedAt, &booking.UpdatedAt)

	return err
}

// GetByID retrieves a booking by ID
func (r *BookingRepository) GetByID(bookingID string) (*models.Booking, error) {
	query := `
		SELECT id, scheduled_trip_id, user_id, booking_reference,
			   number_of_seats, boarding_stop_id, alighting_stop_id,
			   total_fare, payment_status, payment_method, payment_reference, paid_at,
			   booking_status, cancelled_at, cancellation_reason,
			   passenger_name, passenger_phone, passenger_email,
			   special_requests, created_at, updated_at
		FROM bookings
		WHERE id = $1
	`

	return r.scanBooking(r.db.QueryRow(query, bookingID))
}

// GetByReference retrieves a booking by booking reference
func (r *BookingRepository) GetByReference(reference string) (*models.Booking, error) {
	query := `
		SELECT id, scheduled_trip_id, user_id, booking_reference,
			   number_of_seats, boarding_stop_id, alighting_stop_id,
			   total_fare, payment_status, payment_method, payment_reference, paid_at,
			   booking_status, cancelled_at, cancellation_reason,
			   passenger_name, passenger_phone, passenger_email,
			   special_requests, created_at, updated_at
		FROM bookings
		WHERE booking_reference = $1
	`

	return r.scanBooking(r.db.QueryRow(query, reference))
}

// GetByUserID retrieves all bookings for a user
func (r *BookingRepository) GetByUserID(userID string) ([]models.Booking, error) {
	query := `
		SELECT id, scheduled_trip_id, user_id, booking_reference,
			   number_of_seats, boarding_stop_id, alighting_stop_id,
			   total_fare, payment_status, payment_method, payment_reference, paid_at,
			   booking_status, cancelled_at, cancellation_reason,
			   passenger_name, passenger_phone, passenger_email,
			   special_requests, created_at, updated_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanBookings(rows)
}

// GetByScheduledTripID retrieves all bookings for a scheduled trip
func (r *BookingRepository) GetByScheduledTripID(scheduledTripID string) ([]models.Booking, error) {
	query := `
		SELECT id, scheduled_trip_id, user_id, booking_reference,
			   number_of_seats, boarding_stop_id, alighting_stop_id,
			   total_fare, payment_status, payment_method, payment_reference, paid_at,
			   booking_status, cancelled_at, cancellation_reason,
			   passenger_name, passenger_phone, passenger_email,
			   special_requests, created_at, updated_at
		FROM bookings
		WHERE scheduled_trip_id = $1
		  AND booking_status != 'cancelled'
		ORDER BY created_at
	`

	rows, err := r.db.Query(query, scheduledTripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanBookings(rows)
}

// Update updates a booking
func (r *BookingRepository) Update(booking *models.Booking) error {
	query := `
		UPDATE bookings
		SET payment_status = $2, payment_method = $3, payment_reference = $4,
			paid_at = $5, booking_status = $6, cancelled_at = $7,
			cancellation_reason = $8, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		booking.ID, booking.PaymentStatus, booking.PaymentMethod, booking.PaymentReference,
		booking.PaidAt, booking.BookingStatus, booking.CancelledAt,
		booking.CancellationReason,
	).Scan(&booking.UpdatedAt)

	return err
}

// UpdatePaymentStatus updates the payment status of a booking
func (r *BookingRepository) UpdatePaymentStatus(bookingID string, status models.PaymentStatus, method, reference *string) error {
	query := `
		UPDATE bookings
		SET payment_status = $2, payment_method = $3, payment_reference = $4,
			paid_at = CASE WHEN $2 = 'paid' THEN NOW() ELSE paid_at END,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(query, bookingID, status, method, reference)
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

// UpdateBookingStatus updates the booking status
func (r *BookingRepository) UpdateBookingStatus(bookingID string, status models.BookingStatus) error {
	query := `
		UPDATE bookings
		SET booking_status = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(query, bookingID, status)
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

// Cancel cancels a booking
func (r *BookingRepository) Cancel(bookingID string, reason *string) error {
	query := `
		UPDATE bookings
		SET booking_status = 'cancelled',
			cancellation_reason = $2,
			cancelled_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(query, bookingID, reason)
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

// GetTotalBookedSeats returns the total number of booked seats for a scheduled trip
func (r *BookingRepository) GetTotalBookedSeats(scheduledTripID string) (int, error) {
	query := `
		SELECT COALESCE(SUM(number_of_seats), 0)
		FROM bookings
		WHERE scheduled_trip_id = $1
		  AND booking_status = 'confirmed'
		  AND payment_status = 'paid'
	`

	var totalSeats int
	err := r.db.QueryRow(query, scheduledTripID).Scan(&totalSeats)
	return totalSeats, err
}

// scanBooking scans a single booking
func (r *BookingRepository) scanBooking(row scanner) (*models.Booking, error) {
	booking := &models.Booking{}
	var boardingStopID sql.NullString
	var alightingStopID sql.NullString
	var paymentMethod sql.NullString
	var paymentReference sql.NullString
	var paidAt sql.NullTime
	var cancelledAt sql.NullTime
	var cancellationReason sql.NullString
	var passengerName sql.NullString
	var passengerPhone sql.NullString
	var passengerEmail sql.NullString
	var specialRequests sql.NullString

	err := row.Scan(
		&booking.ID, &booking.ScheduledTripID, &booking.UserID, &booking.BookingReference,
		&booking.NumberOfSeats, &boardingStopID, &alightingStopID,
		&booking.TotalFare, &booking.PaymentStatus, &paymentMethod, &paymentReference, &paidAt,
		&booking.BookingStatus, &cancelledAt, &cancellationReason,
		&passengerName, &passengerPhone, &passengerEmail,
		&specialRequests, &booking.CreatedAt, &booking.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if boardingStopID.Valid {
		booking.BoardingStopID = &boardingStopID.String
	}
	if alightingStopID.Valid {
		booking.AlightingStopID = &alightingStopID.String
	}
	if paymentMethod.Valid {
		booking.PaymentMethod = &paymentMethod.String
	}
	if paymentReference.Valid {
		booking.PaymentReference = &paymentReference.String
	}
	if paidAt.Valid {
		booking.PaidAt = &paidAt.Time
	}
	if cancelledAt.Valid {
		booking.CancelledAt = &cancelledAt.Time
	}
	if cancellationReason.Valid {
		booking.CancellationReason = &cancellationReason.String
	}
	if passengerName.Valid {
		booking.PassengerName = &passengerName.String
	}
	if passengerPhone.Valid {
		booking.PassengerPhone = &passengerPhone.String
	}
	if passengerEmail.Valid {
		booking.PassengerEmail = &passengerEmail.String
	}
	if specialRequests.Valid {
		booking.SpecialRequests = &specialRequests.String
	}

	return booking, nil
}

// scanBookings scans multiple bookings from rows
func (r *BookingRepository) scanBookings(rows *sql.Rows) ([]models.Booking, error) {
	bookings := []models.Booking{}

	for rows.Next() {
		var booking models.Booking
		var boardingStopID sql.NullString
		var alightingStopID sql.NullString
		var paymentMethod sql.NullString
		var paymentReference sql.NullString
		var paidAt sql.NullTime
		var cancelledAt sql.NullTime
		var cancellationReason sql.NullString
		var passengerName sql.NullString
		var passengerPhone sql.NullString
		var passengerEmail sql.NullString
		var specialRequests sql.NullString

		err := rows.Scan(
			&booking.ID, &booking.ScheduledTripID, &booking.UserID, &booking.BookingReference,
			&booking.NumberOfSeats, &boardingStopID, &alightingStopID,
			&booking.TotalFare, &booking.PaymentStatus, &paymentMethod, &paymentReference, &paidAt,
			&booking.BookingStatus, &cancelledAt, &cancellationReason,
			&passengerName, &passengerPhone, &passengerEmail,
			&specialRequests, &booking.CreatedAt, &booking.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if boardingStopID.Valid {
			booking.BoardingStopID = &boardingStopID.String
		}
		if alightingStopID.Valid {
			booking.AlightingStopID = &alightingStopID.String
		}
		if paymentMethod.Valid {
			booking.PaymentMethod = &paymentMethod.String
		}
		if paymentReference.Valid {
			booking.PaymentReference = &paymentReference.String
		}
		if paidAt.Valid {
			booking.PaidAt = &paidAt.Time
		}
		if cancelledAt.Valid {
			booking.CancelledAt = &cancelledAt.Time
		}
		if cancellationReason.Valid {
			booking.CancellationReason = &cancellationReason.String
		}
		if passengerName.Valid {
			booking.PassengerName = &passengerName.String
		}
		if passengerPhone.Valid {
			booking.PassengerPhone = &passengerPhone.String
		}
		if passengerEmail.Valid {
			booking.PassengerEmail = &passengerEmail.String
		}
		if specialRequests.Valid {
			booking.SpecialRequests = &specialRequests.String
		}

		bookings = append(bookings, booking)
	}

	return bookings, rows.Err()
}
