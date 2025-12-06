package database

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// AppBookingRepository handles booking database operations
type AppBookingRepository struct {
	db *sqlx.DB
}

// NewAppBookingRepository creates a new AppBookingRepository
func NewAppBookingRepository(db *sqlx.DB) *AppBookingRepository {
	return &AppBookingRepository{db: db}
}

// ============================================================================
// REFERENCE/QR GENERATION FUNCTIONS
// ============================================================================

// GenerateBookingReference generates a unique booking reference
// Format: BL-YYYYMMDD-XXXXXX (6 char alphanumeric)
// Example: BL-20251206-A1B2C3
func (r *AppBookingRepository) GenerateBookingReference() (string, error) {
	todayStr := time.Now().Format("20060102")

	for attempts := 0; attempts < 10; attempts++ {
		// Generate 6 random bytes and take first 6 hex chars
		randomBytes := make([]byte, 3)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		randomStr := strings.ToUpper(hex.EncodeToString(randomBytes))

		newRef := fmt.Sprintf("BL-%s-%s", todayStr, randomStr)

		// Check if exists
		var count int
		err := r.db.Get(&count, `SELECT COUNT(*) FROM bookings WHERE booking_reference = $1`, newRef)
		if err != nil {
			return "", fmt.Errorf("failed to check reference uniqueness: %w", err)
		}

		if count == 0 {
			return newRef, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique booking reference after 10 attempts")
}

// GenerateBusBookingQR generates a unique QR code for bus booking
// Format: QR-YYYYMMDDHHMMSS-XXXXXXXX (8 char alphanumeric)
// Example: QR-20251206143022-A1B2C3D4
func (r *AppBookingRepository) GenerateBusBookingQR() (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		// Generate 8 random bytes and take first 8 hex chars
		randomBytes := make([]byte, 4)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		randomStr := strings.ToUpper(hex.EncodeToString(randomBytes))

		timestampStr := time.Now().Format("20060102150405")
		qrData := fmt.Sprintf("QR-%s-%s", timestampStr, randomStr)

		// Check if exists
		var count int
		err := r.db.Get(&count, `SELECT COUNT(*) FROM bus_bookings WHERE qr_code_data = $1`, qrData)
		if err != nil {
			return "", fmt.Errorf("failed to check QR uniqueness: %w", err)
		}

		if count == 0 {
			return qrData, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique QR code after 10 attempts")
}

// ============================================================================
// MASTER BOOKING OPERATIONS
// ============================================================================

// CreateBooking creates a new master booking with bus booking and seats in a transaction
func (r *AppBookingRepository) CreateBooking(
	booking *models.MasterBooking,
	busBooking *models.BusBooking,
	seats []models.BusBookingSeat,
	tripSeatRepo *TripSeatRepository,
) (*models.BookingResponse, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Generate booking reference (use Go function, not DB function)
	bookingRef, err := r.GenerateBookingReference()
	if err != nil {
		return nil, fmt.Errorf("failed to generate booking reference: %w", err)
	}
	booking.BookingReference = bookingRef

	// 2. Insert master booking
	// Handle device_info JSON serialization
	var deviceInfoJSON interface{}
	if booking.DeviceInfo != nil && len(booking.DeviceInfo) > 0 {
		jsonBytes, err := json.Marshal(booking.DeviceInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize device_info: %w", err)
		}
		deviceInfoJSON = string(jsonBytes)
	}

	bookingQuery := `
		INSERT INTO bookings (
			booking_reference, user_id, booking_type,
			bus_total, lounge_total, pre_order_total,
			subtotal, discount_amount, tax_amount, total_amount,
			promo_code, promo_discount_type, promo_discount_value,
			payment_status, payment_method, booking_status,
			passenger_name, passenger_phone, passenger_email,
			booking_source, device_info, notes
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		) RETURNING id, created_at, updated_at`

	err = tx.QueryRowx(bookingQuery,
		booking.BookingReference, booking.UserID, booking.BookingType,
		booking.BusTotal, booking.LoungeTotal, booking.PreOrderTotal,
		booking.Subtotal, booking.DiscountAmount, booking.TaxAmount, booking.TotalAmount,
		booking.PromoCode, booking.PromoDiscountType, booking.PromoDiscountValue,
		booking.PaymentStatus, booking.PaymentMethod, booking.BookingStatus,
		booking.PassengerName, booking.PassengerPhone, booking.PassengerEmail,
		booking.BookingSource, deviceInfoJSON, booking.Notes,
	).Scan(&booking.ID, &booking.CreatedAt, &booking.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	// 3. Generate QR code for bus booking (use Go function, not DB function)
	qrCode, err := r.GenerateBusBookingQR()
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}
	busBooking.QRCodeData = &qrCode
	now := time.Now()
	busBooking.QRGeneratedAt = &now

	// 4. Insert bus booking
	busBooking.BookingID = booking.ID
	busBookingQuery := `
		INSERT INTO bus_bookings (
			booking_id, scheduled_trip_id,
			route_name, bus_number, bus_type, bus_owner_id,
			boarding_stop_id, boarding_stop_name, boarding_stop_order,
			alighting_stop_id, alighting_stop_name, alighting_stop_order,
			departure_datetime, estimated_arrival_datetime,
			number_of_seats, fare_per_seat, total_fare,
			status, qr_code_data, qr_generated_at, special_requests
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		) RETURNING id, created_at, updated_at`

	err = tx.QueryRowx(busBookingQuery,
		busBooking.BookingID, busBooking.ScheduledTripID,
		busBooking.RouteName, busBooking.BusNumber, busBooking.BusType, busBooking.BusOwnerID,
		busBooking.BoardingStopID, busBooking.BoardingStopName, busBooking.BoardingStopOrder,
		busBooking.AlightingStopID, busBooking.AlightingStopName, busBooking.AlightingStopOrder,
		busBooking.DepartureDatetime, busBooking.EstimatedArrivalDatetime,
		busBooking.NumberOfSeats, busBooking.FarePerSeat, busBooking.TotalFare,
		busBooking.Status, busBooking.QRCodeData, busBooking.QRGeneratedAt, busBooking.SpecialRequests,
	).Scan(&busBooking.ID, &busBooking.CreatedAt, &busBooking.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create bus booking: %w", err)
	}

	// 5. Insert bus booking seats and update trip_seats
	createdSeats := make([]models.BusBookingSeat, 0, len(seats))
	for i := range seats {
		seats[i].BusBookingID = busBooking.ID
		seats[i].ScheduledTripID = busBooking.ScheduledTripID

		seatQuery := `
			INSERT INTO bus_booking_seats (
				bus_booking_id, scheduled_trip_id, trip_seat_id,
				seat_number, seat_type, seat_price,
				passenger_name, passenger_phone, passenger_email,
				passenger_age, passenger_gender, passenger_nic,
				is_primary_passenger, status
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
			) RETURNING id, created_at, updated_at`

		err = tx.QueryRowx(seatQuery,
			seats[i].BusBookingID, seats[i].ScheduledTripID, seats[i].TripSeatID,
			seats[i].SeatNumber, seats[i].SeatType, seats[i].SeatPrice,
			seats[i].PassengerName, seats[i].PassengerPhone, seats[i].PassengerEmail,
			seats[i].PassengerAge, seats[i].PassengerGender, seats[i].PassengerNIC,
			seats[i].IsPrimaryPassenger, seats[i].Status,
		).Scan(&seats[i].ID, &seats[i].CreatedAt, &seats[i].UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to create seat booking for seat %s: %w", seats[i].SeatNumber, err)
		}

		// Update trip_seats to mark as booked (trigger should handle this, but let's be explicit)
		if seats[i].TripSeatID != nil {
			_, err = tx.Exec(`
				UPDATE trip_seats 
				SET status = 'booked', 
				    booking_type = 'app', 
				    bus_booking_seat_id = $1,
				    updated_at = now()
				WHERE id = $2`,
				seats[i].ID, *seats[i].TripSeatID)
			if err != nil {
				return nil, fmt.Errorf("failed to update trip seat %s: %w", seats[i].SeatNumber, err)
			}
		}

		createdSeats = append(createdSeats, seats[i])
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.BookingResponse{
		Booking:    booking,
		BusBooking: busBooking,
		Seats:      createdSeats,
		QRCode:     qrCode,
	}, nil
}

// GetBookingByID retrieves a booking by ID with all related data
func (r *AppBookingRepository) GetBookingByID(bookingID string) (*models.MasterBooking, error) {
	booking := &models.MasterBooking{}
	query := `
		SELECT id, booking_reference, user_id, booking_type,
		       bus_total, lounge_total, pre_order_total,
		       subtotal, discount_amount, tax_amount, total_amount,
		       promo_code, promo_discount_type, promo_discount_value,
		       payment_status, payment_method, payment_reference, payment_gateway, paid_at,
		       booking_status, passenger_name, passenger_phone, passenger_email,
		       confirmed_at, cancelled_at, cancellation_reason, cancelled_by_user_id,
		       completed_at, refund_amount, refund_reference, refunded_at,
		       booking_source, device_info, notes, created_at, updated_at
		FROM bookings WHERE id = $1`

	err := r.db.Get(booking, query, bookingID)
	if err != nil {
		return nil, err
	}

	// Get bus booking if exists
	busBooking, err := r.GetBusBookingByBookingID(bookingID)
	if err == nil {
		booking.BusBooking = busBooking
	}

	return booking, nil
}

// GetBookingByReference retrieves a booking by reference
func (r *AppBookingRepository) GetBookingByReference(reference string) (*models.MasterBooking, error) {
	booking := &models.MasterBooking{}
	query := `
		SELECT id, booking_reference, user_id, booking_type,
		       bus_total, lounge_total, pre_order_total,
		       subtotal, discount_amount, tax_amount, total_amount,
		       promo_code, promo_discount_type, promo_discount_value,
		       payment_status, payment_method, payment_reference, payment_gateway, paid_at,
		       booking_status, passenger_name, passenger_phone, passenger_email,
		       confirmed_at, cancelled_at, cancellation_reason, cancelled_by_user_id,
		       completed_at, refund_amount, refund_reference, refunded_at,
		       booking_source, device_info, notes, created_at, updated_at
		FROM bookings WHERE booking_reference = $1`

	err := r.db.Get(booking, query, reference)
	if err != nil {
		return nil, err
	}

	// Get bus booking if exists
	busBooking, err := r.GetBusBookingByBookingID(booking.ID)
	if err == nil {
		booking.BusBooking = busBooking
	}

	return booking, nil
}

// GetBookingsByUserID retrieves all bookings for a user
func (r *AppBookingRepository) GetBookingsByUserID(userID string, limit, offset int) ([]models.BookingListItem, error) {
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			b.total_amount, b.payment_status, b.booking_status,
			b.passenger_name, b.created_at,
			bb.route_name, bb.departure_datetime, bb.number_of_seats,
			bb.status as bus_status, bb.qr_code_data
		FROM bookings b
		LEFT JOIN bus_bookings bb ON bb.booking_id = b.id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	var bookings []models.BookingListItem
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// GetUpcomingBookingsByUserID retrieves upcoming bookings for a user
func (r *AppBookingRepository) GetUpcomingBookingsByUserID(userID string) ([]models.BookingListItem, error) {
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			b.total_amount, b.payment_status, b.booking_status,
			b.passenger_name, b.created_at,
			bb.route_name, bb.departure_datetime, bb.number_of_seats,
			bb.status as bus_status, bb.qr_code_data
		FROM bookings b
		LEFT JOIN bus_bookings bb ON bb.booking_id = b.id
		WHERE b.user_id = $1
		  AND b.booking_status IN ('pending', 'confirmed', 'in_progress')
		  AND (bb.departure_datetime IS NULL OR bb.departure_datetime >= NOW())
		ORDER BY bb.departure_datetime ASC`

	var bookings []models.BookingListItem
	err := r.db.Select(&bookings, query, userID)
	return bookings, err
}

// UpdatePaymentStatus updates payment status for a booking
func (r *AppBookingRepository) UpdatePaymentStatus(
	bookingID string,
	status models.MasterPaymentStatus,
	method, reference, gateway *string,
) error {
	query := `
		UPDATE bookings 
		SET payment_status = $1,
		    payment_method = COALESCE($2, payment_method),
		    payment_reference = COALESCE($3, payment_reference),
		    payment_gateway = COALESCE($4, payment_gateway),
		    paid_at = CASE WHEN $1 = 'paid' THEN NOW() ELSE paid_at END,
		    confirmed_at = CASE WHEN $1 = 'paid' THEN COALESCE(confirmed_at, NOW()) ELSE confirmed_at END,
		    booking_status = CASE WHEN $1 = 'paid' THEN 'confirmed' ELSE booking_status END,
		    updated_at = NOW()
		WHERE id = $5`

	_, err := r.db.Exec(query, status, method, reference, gateway, bookingID)
	return err
}

// CancelBooking cancels a booking and releases seats
func (r *AppBookingRepository) CancelBooking(bookingID, userID string, reason *string) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Update master booking
	_, err = tx.Exec(`
		UPDATE bookings 
		SET booking_status = 'cancelled',
		    cancelled_at = NOW(),
		    cancelled_by_user_id = $1,
		    cancellation_reason = $2,
		    updated_at = NOW()
		WHERE id = $3`,
		userID, reason, bookingID)
	if err != nil {
		return fmt.Errorf("failed to cancel booking: %w", err)
	}

	// 2. Update bus booking status
	_, err = tx.Exec(`
		UPDATE bus_bookings 
		SET status = 'cancelled',
		    cancelled_at = NOW(),
		    cancellation_reason = $1,
		    updated_at = NOW()
		WHERE booking_id = $2`,
		reason, bookingID)
	if err != nil {
		return fmt.Errorf("failed to cancel bus booking: %w", err)
	}

	// 3. Update seat statuses
	_, err = tx.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'cancelled',
		    cancelled_at = NOW(),
		    updated_at = NOW()
		WHERE bus_booking_id IN (SELECT id FROM bus_bookings WHERE booking_id = $1)`,
		bookingID)
	if err != nil {
		return fmt.Errorf("failed to cancel seat bookings: %w", err)
	}

	// 4. Release trip_seats (the trigger should handle this, but let's be explicit)
	_, err = tx.Exec(`
		UPDATE trip_seats 
		SET status = 'available',
		    booking_type = NULL,
		    bus_booking_seat_id = NULL,
		    updated_at = NOW()
		WHERE bus_booking_seat_id IN (
			SELECT bbs.id FROM bus_booking_seats bbs
			JOIN bus_bookings bb ON bb.id = bbs.bus_booking_id
			WHERE bb.booking_id = $1
		)`,
		bookingID)
	if err != nil {
		return fmt.Errorf("failed to release trip seats: %w", err)
	}

	return tx.Commit()
}

// ============================================================================
// BUS BOOKING OPERATIONS
// ============================================================================

// GetBusBookingByBookingID retrieves bus booking by master booking ID
func (r *AppBookingRepository) GetBusBookingByBookingID(bookingID string) (*models.BusBooking, error) {
	busBooking := &models.BusBooking{}
	query := `
		SELECT id, booking_id, scheduled_trip_id,
		       route_name, bus_number, bus_type, bus_owner_id,
		       boarding_stop_id, boarding_stop_name, boarding_stop_order,
		       alighting_stop_id, alighting_stop_name, alighting_stop_order,
		       departure_datetime, estimated_arrival_datetime,
		       actual_departure_datetime, actual_arrival_datetime,
		       number_of_seats, fare_per_seat, total_fare,
		       status, checked_in_at, checked_in_by_user_id,
		       boarded_at, boarded_by_user_id, completed_at,
		       cancelled_at, cancellation_reason,
		       qr_code_data, qr_generated_at, special_requests,
		       created_at, updated_at
		FROM bus_bookings WHERE booking_id = $1`

	err := r.db.Get(busBooking, query, bookingID)
	if err != nil {
		return nil, err
	}

	// Get seats
	seats, err := r.GetSeatsByBusBookingID(busBooking.ID)
	if err == nil {
		busBooking.Seats = seats
	}

	return busBooking, nil
}

// GetBusBookingByQRCode retrieves bus booking by QR code
func (r *AppBookingRepository) GetBusBookingByQRCode(qrCode string) (*models.BusBooking, error) {
	busBooking := &models.BusBooking{}
	query := `
		SELECT id, booking_id, scheduled_trip_id,
		       route_name, bus_number, bus_type, bus_owner_id,
		       boarding_stop_id, boarding_stop_name, boarding_stop_order,
		       alighting_stop_id, alighting_stop_name, alighting_stop_order,
		       departure_datetime, estimated_arrival_datetime,
		       actual_departure_datetime, actual_arrival_datetime,
		       number_of_seats, fare_per_seat, total_fare,
		       status, checked_in_at, checked_in_by_user_id,
		       boarded_at, boarded_by_user_id, completed_at,
		       cancelled_at, cancellation_reason,
		       qr_code_data, qr_generated_at, special_requests,
		       created_at, updated_at
		FROM bus_bookings WHERE qr_code_data = $1`

	err := r.db.Get(busBooking, query, qrCode)
	if err != nil {
		return nil, err
	}

	// Get seats
	seats, err := r.GetSeatsByBusBookingID(busBooking.ID)
	if err == nil {
		busBooking.Seats = seats
	}

	return busBooking, nil
}

// GetBusBookingsByTripID retrieves all bus bookings for a scheduled trip
func (r *AppBookingRepository) GetBusBookingsByTripID(tripID string) ([]models.BusBooking, error) {
	query := `
		SELECT id, booking_id, scheduled_trip_id,
		       route_name, bus_number, bus_type, bus_owner_id,
		       boarding_stop_id, boarding_stop_name, boarding_stop_order,
		       alighting_stop_id, alighting_stop_name, alighting_stop_order,
		       departure_datetime, estimated_arrival_datetime,
		       actual_departure_datetime, actual_arrival_datetime,
		       number_of_seats, fare_per_seat, total_fare,
		       status, checked_in_at, checked_in_by_user_id,
		       boarded_at, boarded_by_user_id, completed_at,
		       cancelled_at, cancellation_reason,
		       qr_code_data, qr_generated_at, special_requests,
		       created_at, updated_at
		FROM bus_bookings 
		WHERE scheduled_trip_id = $1 AND status != 'cancelled'
		ORDER BY created_at DESC`

	var bookings []models.BusBooking
	err := r.db.Select(&bookings, query, tripID)
	return bookings, err
}

// UpdateBusBookingStatus updates bus booking status (check-in, board, complete)
func (r *AppBookingRepository) UpdateBusBookingStatus(
	busBookingID string,
	status models.BusBookingStatus,
	staffUserID *string,
) error {
	var query string
	switch status {
	case models.BusBookingCheckedIn:
		query = `UPDATE bus_bookings SET status = $1, checked_in_at = NOW(), checked_in_by_user_id = $2, updated_at = NOW() WHERE id = $3`
	case models.BusBookingBoarded:
		query = `UPDATE bus_bookings SET status = $1, boarded_at = NOW(), boarded_by_user_id = $2, updated_at = NOW() WHERE id = $3`
	case models.BusBookingCompleted:
		query = `UPDATE bus_bookings SET status = $1, completed_at = NOW(), updated_at = NOW() WHERE id = $3`
	default:
		query = `UPDATE bus_bookings SET status = $1, updated_at = NOW() WHERE id = $3`
	}

	_, err := r.db.Exec(query, status, staffUserID, busBookingID)
	return err
}

// ============================================================================
// SEAT OPERATIONS
// ============================================================================

// GetSeatsByBusBookingID retrieves seats for a bus booking
func (r *AppBookingRepository) GetSeatsByBusBookingID(busBookingID string) ([]models.BusBookingSeat, error) {
	query := `
		SELECT id, bus_booking_id, scheduled_trip_id, trip_seat_id,
		       seat_number, seat_type, seat_price,
		       passenger_name, passenger_phone, passenger_email,
		       passenger_age, passenger_gender, passenger_nic,
		       is_primary_passenger, status,
		       checked_in_at, boarded_at, cancelled_at,
		       created_at, updated_at
		FROM bus_booking_seats
		WHERE bus_booking_id = $1
		ORDER BY seat_number`

	var seats []models.BusBookingSeat
	err := r.db.Select(&seats, query, busBookingID)
	return seats, err
}

// CheckSeatAvailability checks if seats are available for booking
func (r *AppBookingRepository) CheckSeatAvailability(tripSeatIDs []string) ([]models.TripSeat, error) {
	if len(tripSeatIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
		       seat_price, status, booking_type
		FROM trip_seats
		WHERE id = ANY($1)`

	var seats []models.TripSeat
	err := r.db.Select(&seats, query, tripSeatIDs)
	if err != nil {
		return nil, err
	}

	// Check all seats are available
	for _, seat := range seats {
		if seat.Status != models.TripSeatStatusAvailable {
			return nil, fmt.Errorf("seat %s is not available (status: %s)", seat.SeatNumber, seat.Status)
		}
	}

	return seats, nil
}

// CountBookingsByTripID counts confirmed bookings for a trip
func (r *AppBookingRepository) CountBookingsByTripID(tripID string) (int, error) {
	var count int
	err := r.db.Get(&count, `
		SELECT COUNT(*) FROM bus_bookings 
		WHERE scheduled_trip_id = $1 AND status NOT IN ('cancelled')`,
		tripID)
	return count, err
}

// CountSeatsByTripID counts booked seats for a trip
func (r *AppBookingRepository) CountSeatsByTripID(tripID string) (int, error) {
	var count int
	err := r.db.Get(&count, `
		SELECT COUNT(*) FROM bus_booking_seats 
		WHERE scheduled_trip_id = $1 AND status NOT IN ('cancelled')`,
		tripID)
	return count, err
}

// ============================================================================
// STAFF OPERATIONS (for conductor/driver app)
// ============================================================================

// GetBookingsForTrip retrieves all bookings for a trip (for staff to manage)
func (r *AppBookingRepository) GetBookingsForTrip(tripID string) ([]models.BusBooking, error) {
	return r.GetBusBookingsByTripID(tripID)
}

// CheckInBusBooking marks a bus booking as checked in (all seats)
func (r *AppBookingRepository) CheckInBusBooking(busBookingID, staffUserID string) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update bus booking
	_, err = tx.Exec(`
		UPDATE bus_bookings 
		SET status = 'checked_in', 
		    checked_in_at = NOW(),
		    checked_in_by_user_id = $1,
		    updated_at = NOW()
		WHERE id = $2`,
		staffUserID, busBookingID)
	if err != nil {
		return err
	}

	// Update all seats
	_, err = tx.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'checked_in',
		    checked_in_at = NOW(),
		    updated_at = NOW()
		WHERE bus_booking_id = $1 AND status = 'booked'`,
		busBookingID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CheckInPassenger marks a specific seat as checked in
func (r *AppBookingRepository) CheckInPassenger(seatID, staffUserID string) error {
	_, err := r.db.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'checked_in',
		    checked_in_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`,
		seatID)
	return err
}

// BoardPassenger marks a specific seat as boarded
func (r *AppBookingRepository) BoardPassenger(seatID, staffUserID string) error {
	_, err := r.db.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'boarded',
		    boarded_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`,
		seatID)
	return err
}

// MarkNoShow marks a specific seat as no-show
func (r *AppBookingRepository) MarkNoShow(seatID, staffUserID string) error {
	_, err := r.db.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'no_show',
		    updated_at = NOW()
		WHERE id = $1`,
		seatID)
	return err
}
