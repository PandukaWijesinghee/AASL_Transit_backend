package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BookingIntentRepository handles booking intent database operations
type BookingIntentRepository struct {
	db *sqlx.DB
}

// NewBookingIntentRepository creates a new BookingIntentRepository
func NewBookingIntentRepository(db *sqlx.DB) *BookingIntentRepository {
	return &BookingIntentRepository{db: db}
}

// ============================================================================
// BOOKING INTENT CRUD OPERATIONS
// ============================================================================

// CreateIntent creates a new booking intent
func (r *BookingIntentRepository) CreateIntent(intent *models.BookingIntent) error {
	intent.ID = uuid.New()
	intent.CreatedAt = time.Now()
	intent.UpdatedAt = time.Now()

	// Marshal JSONB fields
	var busIntentJSON, preLoungeJSON, postLoungeJSON, pricingSnapshotJSON interface{}
	var err error

	if intent.BusIntent != nil {
		busIntentJSON, err = json.Marshal(intent.BusIntent)
		if err != nil {
			return fmt.Errorf("failed to marshal bus_intent: %w", err)
		}
	}
	if intent.PreTripLoungeIntent != nil {
		preLoungeJSON, err = json.Marshal(intent.PreTripLoungeIntent)
		if err != nil {
			return fmt.Errorf("failed to marshal pre_trip_lounge_intent: %w", err)
		}
	}
	if intent.PostTripLoungeIntent != nil {
		postLoungeJSON, err = json.Marshal(intent.PostTripLoungeIntent)
		if err != nil {
			return fmt.Errorf("failed to marshal post_trip_lounge_intent: %w", err)
		}
	}
	pricingSnapshotJSON, err = json.Marshal(intent.PricingSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing_snapshot: %w", err)
	}

	query := `
		INSERT INTO booking_intents (
			id, user_id, intent_type, status,
			bus_intent, pre_trip_lounge_intent, post_trip_lounge_intent,
			bus_fare, pre_lounge_fare, post_lounge_fare, total_amount, currency,
			pricing_snapshot, payment_gateway, expires_at,
			idempotency_key, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)`

	_, err = r.db.Exec(query,
		intent.ID, intent.UserID, intent.IntentType, intent.Status,
		busIntentJSON, preLoungeJSON, postLoungeJSON,
		intent.BusFare, intent.PreLoungeFare, intent.PostLoungeFare, intent.TotalAmount, intent.Currency,
		pricingSnapshotJSON, intent.PaymentGateway, intent.ExpiresAt,
		intent.IdempotencyKey, intent.CreatedAt, intent.UpdatedAt,
	)
	return err
}

// GetIntentByID retrieves an intent by ID
func (r *BookingIntentRepository) GetIntentByID(intentID uuid.UUID) (*models.BookingIntent, error) {
	var intent models.BookingIntent
	var busIntentJSON, preLoungeJSON, postLoungeJSON, pricingSnapshotJSON sql.NullString
	var paymentStatus sql.NullString

	query := `
		SELECT 
			id, user_id, intent_type, status,
			bus_intent, pre_trip_lounge_intent, post_trip_lounge_intent,
			bus_fare, pre_lounge_fare, post_lounge_fare, total_amount, currency,
			pricing_snapshot, payment_reference, payment_status, payment_gateway,
			bus_booking_id, pre_lounge_booking_id, post_lounge_booking_id,
			expires_at, payment_initiated_at, confirmed_at, expired_at,
			created_at, updated_at, idempotency_key
		FROM booking_intents
		WHERE id = $1`

	err := r.db.QueryRow(query, intentID).Scan(
		&intent.ID, &intent.UserID, &intent.IntentType, &intent.Status,
		&busIntentJSON, &preLoungeJSON, &postLoungeJSON,
		&intent.BusFare, &intent.PreLoungeFare, &intent.PostLoungeFare, &intent.TotalAmount, &intent.Currency,
		&pricingSnapshotJSON, &intent.PaymentReference, &paymentStatus, &intent.PaymentGateway,
		&intent.BusBookingID, &intent.PreLoungeBookingID, &intent.PostLoungeBookingID,
		&intent.ExpiresAt, &intent.PaymentInitiatedAt, &intent.ConfirmedAt, &intent.ExpiredAt,
		&intent.CreatedAt, &intent.UpdatedAt, &intent.IdempotencyKey,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse payment status
	if paymentStatus.Valid {
		ps := models.IntentPaymentStatus(paymentStatus.String)
		intent.PaymentStatus = &ps
	}

	// Unmarshal JSONB fields
	if busIntentJSON.Valid && busIntentJSON.String != "" {
		intent.BusIntent = &models.BusIntentPayload{}
		if err := json.Unmarshal([]byte(busIntentJSON.String), intent.BusIntent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bus_intent: %w", err)
		}
	}
	if preLoungeJSON.Valid && preLoungeJSON.String != "" {
		intent.PreTripLoungeIntent = &models.LoungeIntentPayload{}
		if err := json.Unmarshal([]byte(preLoungeJSON.String), intent.PreTripLoungeIntent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pre_trip_lounge_intent: %w", err)
		}
	}
	if postLoungeJSON.Valid && postLoungeJSON.String != "" {
		intent.PostTripLoungeIntent = &models.LoungeIntentPayload{}
		if err := json.Unmarshal([]byte(postLoungeJSON.String), intent.PostTripLoungeIntent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal post_trip_lounge_intent: %w", err)
		}
	}
	if pricingSnapshotJSON.Valid && pricingSnapshotJSON.String != "" {
		if err := json.Unmarshal([]byte(pricingSnapshotJSON.String), &intent.PricingSnapshot); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pricing_snapshot: %w", err)
		}
	}

	return &intent, nil
}

// GetIntentByIdempotencyKey retrieves an intent by idempotency key
func (r *BookingIntentRepository) GetIntentByIdempotencyKey(key string, userID uuid.UUID) (*models.BookingIntent, error) {
	var intentID uuid.UUID
	query := `SELECT id FROM booking_intents WHERE idempotency_key = $1 AND user_id = $2`
	err := r.db.Get(&intentID, query, key, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetIntentByID(intentID)
}

// GetIntentByPaymentReference retrieves an intent by payment reference
func (r *BookingIntentRepository) GetIntentByPaymentReference(paymentRef string) (*models.BookingIntent, error) {
	var intentID uuid.UUID
	query := `SELECT id FROM booking_intents WHERE payment_reference = $1`
	err := r.db.Get(&intentID, query, paymentRef)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetIntentByID(intentID)
}

// GetIntentsByUserID retrieves all intents for a user
func (r *BookingIntentRepository) GetIntentsByUserID(userID uuid.UUID, limit, offset int) ([]*models.BookingIntent, error) {
	query := `
		SELECT id FROM booking_intents 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`

	var intentIDs []uuid.UUID
	err := r.db.Select(&intentIDs, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	intents := make([]*models.BookingIntent, 0, len(intentIDs))
	for _, id := range intentIDs {
		intent, err := r.GetIntentByID(id)
		if err != nil {
			return nil, err
		}
		if intent != nil {
			intents = append(intents, intent)
		}
	}
	return intents, nil
}

// ============================================================================
// STATUS UPDATE OPERATIONS
// ============================================================================

// UpdateIntentStatus updates the status of an intent
func (r *BookingIntentRepository) UpdateIntentStatus(intentID uuid.UUID, status models.BookingIntentStatus) error {
	query := `UPDATE booking_intents SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, intentID, status)
	return err
}

// UpdateIntentPaymentPending marks intent as payment pending
func (r *BookingIntentRepository) UpdateIntentPaymentPending(intentID uuid.UUID, paymentRef string) error {
	query := `
		UPDATE booking_intents 
		SET status = 'payment_pending', 
		    payment_reference = $2, 
		    payment_status = 'pending',
		    payment_initiated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'held'`
	result, err := r.db.Exec(query, intentID, paymentRef)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("intent not in 'held' status or not found")
	}
	return nil
}

// UpdateIntentPaymentSuccess marks payment as successful
func (r *BookingIntentRepository) UpdateIntentPaymentSuccess(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET payment_status = 'success',
		    updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(query, intentID)
	return err
}

// UpdateIntentConfirmed marks intent as confirmed with booking IDs
func (r *BookingIntentRepository) UpdateIntentConfirmed(
	intentID uuid.UUID,
	busBookingID, preLoungeBookingID, postLoungeBookingID *uuid.UUID,
) error {
	query := `
		UPDATE booking_intents 
		SET status = 'confirmed',
		    bus_booking_id = $2,
		    pre_lounge_booking_id = $3,
		    post_lounge_booking_id = $4,
		    confirmed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('held', 'payment_pending', 'confirming')`
	result, err := r.db.Exec(query, intentID, busBookingID, preLoungeBookingID, postLoungeBookingID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("intent not in valid status for confirmation")
	}
	return nil
}

// UpdateIntentExpired marks intent as expired
func (r *BookingIntentRepository) UpdateIntentExpired(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'expired',
		    expired_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('held', 'payment_pending')`
	_, err := r.db.Exec(query, intentID)
	return err
}

// UpdateIntentCancelled marks intent as cancelled
func (r *BookingIntentRepository) UpdateIntentCancelled(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'cancelled',
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('held', 'payment_pending')`
	_, err := r.db.Exec(query, intentID)
	return err
}

// UpdateIntentConfirmationFailed marks intent as confirmation failed (needs refund)
func (r *BookingIntentRepository) UpdateIntentConfirmationFailed(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'confirmation_failed',
		    updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(query, intentID)
	return err
}

// ============================================================================
// SEAT HOLDING OPERATIONS (TTL-based)
// ============================================================================

// HoldSeatsForIntent locks seats for a booking intent with TTL
// Returns the number of successfully held seats and any error
func (r *BookingIntentRepository) HoldSeatsForIntent(intentID uuid.UUID, seatIDs []string, expiresAt time.Time) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	// Use IN clause with proper binding
	query, args, err := sqlx.In(`
		UPDATE trip_seats 
		SET held_by_intent_id = ?, held_until = ?, updated_at = NOW()
		WHERE id IN (?) 
		  AND status = 'available'
		  AND (held_by_intent_id IS NULL OR held_until < NOW())
	`, intentID, expiresAt, seatIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to build hold query: %w", err)
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to hold seats: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// ReleaseSeatHoldsForIntent releases all seat holds for an intent
func (r *BookingIntentRepository) ReleaseSeatHoldsForIntent(intentID uuid.UUID) error {
	query := `
		UPDATE trip_seats 
		SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW()
		WHERE held_by_intent_id = $1`
	_, err := r.db.Exec(query, intentID)
	return err
}

// GetHeldSeatsForIntent returns all seats held by an intent
func (r *BookingIntentRepository) GetHeldSeatsForIntent(intentID uuid.UUID) ([]models.TripSeat, error) {
	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
		       seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
		       block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE held_by_intent_id = $1 AND held_until > NOW()
		ORDER BY row_number, position`

	var seats []models.TripSeat
	err := r.db.Select(&seats, query, intentID)
	return seats, err
}

// CheckSeatsAvailableForHold checks if seats can be held (not booked, not held by others)
func (r *BookingIntentRepository) CheckSeatsAvailableForHold(seatIDs []string) ([]string, []string, error) {
	if len(seatIDs) == 0 {
		return []string{}, []string{}, nil
	}

	query, args, err := sqlx.In(`
		SELECT id, status, held_by_intent_id, held_until
		FROM trip_seats
		WHERE id IN (?)
	`, seatIDs)
	if err != nil {
		return nil, nil, err
	}

	query = r.db.Rebind(query)

	type seatStatus struct {
		ID             string       `db:"id"`
		Status         string       `db:"status"`
		HeldByIntentID *uuid.UUID   `db:"held_by_intent_id"`
		HeldUntil      *time.Time   `db:"held_until"`
	}

	var seats []seatStatus
	err = r.db.Select(&seats, query, args...)
	if err != nil {
		return nil, nil, err
	}

	available := make([]string, 0)
	unavailable := make([]string, 0)

	for _, seat := range seats {
		// Check if available: status is 'available' AND (no hold OR hold expired)
		if seat.Status == "available" {
			if seat.HeldByIntentID == nil || (seat.HeldUntil != nil && seat.HeldUntil.Before(time.Now())) {
				available = append(available, seat.ID)
			} else {
				unavailable = append(unavailable, seat.ID)
			}
		} else {
			unavailable = append(unavailable, seat.ID)
		}
	}

	return available, unavailable, nil
}

// ============================================================================
// LOUNGE CAPACITY HOLD OPERATIONS
// ============================================================================

// CreateLoungeCapacityHold creates a lounge capacity hold for an intent
func (r *BookingIntentRepository) CreateLoungeCapacityHold(hold *models.LoungeCapacityHold) error {
	hold.ID = uuid.New()
	hold.CreatedAt = time.Now()
	hold.Status = "held"

	query := `
		INSERT INTO lounge_capacity_holds (
			id, lounge_id, intent_id, date, time_slot_start, time_slot_end,
			guests_count, held_until, status, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err := r.db.Exec(query,
		hold.ID, hold.LoungeID, hold.IntentID, hold.Date,
		hold.TimeSlotStart, hold.TimeSlotEnd, hold.GuestsCount,
		hold.HeldUntil, hold.Status, hold.CreatedAt,
	)
	return err
}

// ReleaseLoungeHoldsForIntent releases all lounge holds for an intent
func (r *BookingIntentRepository) ReleaseLoungeHoldsForIntent(intentID uuid.UUID) error {
	query := `
		UPDATE lounge_capacity_holds 
		SET status = 'released'
		WHERE intent_id = $1 AND status = 'held'`
	_, err := r.db.Exec(query, intentID)
	return err
}

// ConfirmLoungeHoldsForIntent marks lounge holds as confirmed
func (r *BookingIntentRepository) ConfirmLoungeHoldsForIntent(intentID uuid.UUID) error {
	query := `
		UPDATE lounge_capacity_holds 
		SET status = 'confirmed'
		WHERE intent_id = $1 AND status = 'held'`
	_, err := r.db.Exec(query, intentID)
	return err
}

// GetLoungeCapacityAvailable calculates available capacity for a lounge at a time
func (r *BookingIntentRepository) GetLoungeCapacityAvailable(
	loungeID uuid.UUID,
	date time.Time,
	timeSlotStart, timeSlotEnd string,
) (int, error) {
	// First, get the lounge max capacity
	var maxCapacity int
	err := r.db.Get(&maxCapacity, `SELECT COALESCE(max_capacity, 50) FROM lounges WHERE id = $1`, loungeID)
	if err != nil {
		return 0, fmt.Errorf("failed to get lounge capacity: %w", err)
	}

	// Count confirmed bookings (overlapping time slot)
	var confirmedCount int
	confirmedQuery := `
		SELECT COALESCE(SUM(number_of_guests), 0) 
		FROM lounge_bookings 
		WHERE lounge_id = $1 
		  AND DATE(scheduled_arrival) = $2
		  AND status IN ('pending', 'confirmed', 'checked_in')
		  AND (
		    (scheduled_arrival::time >= $3::time AND scheduled_arrival::time < $4::time)
		    OR (scheduled_departure::time > $3::time AND scheduled_departure::time <= $4::time)
		    OR (scheduled_arrival::time <= $3::time AND scheduled_departure::time >= $4::time)
		  )`
	err = r.db.Get(&confirmedCount, confirmedQuery, loungeID, date.Format("2006-01-02"), timeSlotStart, timeSlotEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to count confirmed bookings: %w", err)
	}

	// Count active holds (not expired)
	var heldCount int
	heldQuery := `
		SELECT COALESCE(SUM(guests_count), 0) 
		FROM lounge_capacity_holds 
		WHERE lounge_id = $1 
		  AND date = $2
		  AND status = 'held'
		  AND held_until > NOW()
		  AND (
		    (time_slot_start >= $3::time AND time_slot_start < $4::time)
		    OR (time_slot_end > $3::time AND time_slot_end <= $4::time)
		    OR (time_slot_start <= $3::time AND time_slot_end >= $4::time)
		  )`
	err = r.db.Get(&heldCount, heldQuery, loungeID, date.Format("2006-01-02"), timeSlotStart, timeSlotEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to count held capacity: %w", err)
	}

	available := maxCapacity - confirmedCount - heldCount
	if available < 0 {
		available = 0
	}

	return available, nil
}

// ============================================================================
// TTL EXPIRATION (Background Job Support)
// ============================================================================

// GetExpiredHeldIntents returns intents that are held but past their expiry time
func (r *BookingIntentRepository) GetExpiredHeldIntents(limit int) ([]*models.BookingIntent, error) {
	query := `
		SELECT id FROM booking_intents 
		WHERE status = 'held' AND expires_at < NOW()
		LIMIT $1`

	var intentIDs []uuid.UUID
	err := r.db.Select(&intentIDs, query, limit)
	if err != nil {
		return nil, err
	}

	intents := make([]*models.BookingIntent, 0, len(intentIDs))
	for _, id := range intentIDs {
		intent, err := r.GetIntentByID(id)
		if err != nil {
			return nil, err
		}
		if intent != nil {
			intents = append(intents, intent)
		}
	}
	return intents, nil
}

// GetPaymentPendingTimedOutIntents returns payment_pending intents that have timed out
func (r *BookingIntentRepository) GetPaymentPendingTimedOutIntents(timeout time.Duration, limit int) ([]*models.BookingIntent, error) {
	cutoff := time.Now().Add(-timeout)
	query := `
		SELECT id FROM booking_intents 
		WHERE status = 'payment_pending' 
		  AND payment_initiated_at < $1
		LIMIT $2`

	var intentIDs []uuid.UUID
	err := r.db.Select(&intentIDs, query, cutoff, limit)
	if err != nil {
		return nil, err
	}

	intents := make([]*models.BookingIntent, 0, len(intentIDs))
	for _, id := range intentIDs {
		intent, err := r.GetIntentByID(id)
		if err != nil {
			return nil, err
		}
		if intent != nil {
			intents = append(intents, intent)
		}
	}
	return intents, nil
}

// ExpireIntentAndReleaseHolds atomically expires an intent and releases all its holds
func (r *BookingIntentRepository) ExpireIntentAndReleaseHolds(intentID uuid.UUID) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Update intent status to expired
	_, err = tx.Exec(`
		UPDATE booking_intents 
		SET status = 'expired', expired_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status IN ('held', 'payment_pending')
	`, intentID)
	if err != nil {
		return fmt.Errorf("failed to expire intent: %w", err)
	}

	// 2. Release seat holds
	_, err = tx.Exec(`
		UPDATE trip_seats 
		SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW()
		WHERE held_by_intent_id = $1
	`, intentID)
	if err != nil {
		return fmt.Errorf("failed to release seat holds: %w", err)
	}

	// 3. Release lounge holds
	_, err = tx.Exec(`
		UPDATE lounge_capacity_holds 
		SET status = 'released'
		WHERE intent_id = $1 AND status = 'held'
	`, intentID)
	if err != nil {
		return fmt.Errorf("failed to release lounge holds: %w", err)
	}

	return tx.Commit()
}

// ReleaseOrphanSeatHolds releases seat holds where the intent doesn't exist
func (r *BookingIntentRepository) ReleaseOrphanSeatHolds() (int, error) {
	query := `
		UPDATE trip_seats 
		SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW()
		WHERE held_by_intent_id IS NOT NULL 
		  AND held_by_intent_id NOT IN (SELECT id FROM booking_intents)`
	result, err := r.db.Exec(query)
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// ReleaseExpiredSeatHolds releases seat holds that have passed their TTL
func (r *BookingIntentRepository) ReleaseExpiredSeatHolds() (int, error) {
	query := `
		UPDATE trip_seats 
		SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW()
		WHERE held_by_intent_id IS NOT NULL AND held_until < NOW()`
	result, err := r.db.Exec(query)
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// ============================================================================
// TRANSACTION SUPPORT
// ============================================================================

// BeginTx starts a new transaction
func (r *BookingIntentRepository) BeginTx() (*sqlx.Tx, error) {
	return r.db.Beginx()
}

// GetDB returns the underlying database connection
func (r *BookingIntentRepository) GetDB() *sqlx.DB {
	return r.db
}
