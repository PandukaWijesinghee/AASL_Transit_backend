package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// PaymentAuditRepository handles payment audit operations
type PaymentAuditRepository struct {
	db     *sqlx.DB
	logger *logrus.Logger
}

// NewPaymentAuditRepository creates a new payment audit repository
func NewPaymentAuditRepository(db *sqlx.DB, logger *logrus.Logger) *PaymentAuditRepository {
	return &PaymentAuditRepository{
		db:     db,
		logger: logger,
	}
}

// Log creates a new payment audit entry
// This should NEVER fail silently - payment events must be logged
func (r *PaymentAuditRepository) Log(ctx context.Context, audit *models.PaymentAudit) error {
	if audit == nil {
		return fmt.Errorf("audit entry cannot be nil")
	}

	// Ensure ID and timestamp are set
	if audit.ID == uuid.Nil {
		audit.ID = uuid.New()
	}
	if audit.CreatedAt.IsZero() {
		audit.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO payment_audits (
			id, intent_id, payment_uid, payment_reference,
			event_type, event_source,
			expected_amount, received_amount, currency, amounts_match,
			payment_status, gateway_transaction_id,
			request_payload, response_payload, raw_body,
			http_status_code, http_method, endpoint_url,
			error_message, error_code,
			processing_time_ms, is_duplicate, idempotency_key,
			ip_address, user_agent, correlation_id,
			created_at, processed_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8, $9, $10,
			$11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20,
			$21, $22, $23,
			$24, $25, $26,
			$27, $28
		)`

	_, err := r.db.ExecContext(ctx, query,
		audit.ID, audit.IntentID, audit.PaymentUID, audit.PaymentReference,
		audit.EventType, audit.EventSource,
		audit.ExpectedAmount, audit.ReceivedAmount, audit.Currency, audit.AmountsMatch,
		audit.PaymentStatus, audit.GatewayTransactionID,
		audit.RequestPayload, audit.ResponsePayload, audit.RawBody,
		audit.HTTPStatusCode, audit.HTTPMethod, audit.EndpointURL,
		audit.ErrorMessage, audit.ErrorCode,
		audit.ProcessingTimeMs, audit.IsDuplicate, audit.IdempotencyKey,
		audit.IPAddress, audit.UserAgent, audit.CorrelationID,
		audit.CreatedAt, audit.ProcessedAt,
	)

	if err != nil {
		r.logger.WithError(err).WithFields(logrus.Fields{
			"event_type":  audit.EventType,
			"payment_uid": audit.PaymentUID,
		}).Error("CRITICAL: Failed to log payment audit - THIS SHOULD NEVER HAPPEN")
		return fmt.Errorf("failed to log payment audit: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"audit_id":    audit.ID,
		"event_type":  audit.EventType,
		"payment_uid": audit.PaymentUID,
	}).Debug("Payment audit logged")

	return nil
}

// CheckDuplicate checks if a webhook event has already been processed
// Returns true if duplicate, false if new
func (r *PaymentAuditRepository) CheckDuplicate(ctx context.Context, paymentUID string, eventType models.PaymentEventType, idempotencyKey string) (bool, error) {
	if idempotencyKey == "" {
		// Generate idempotency key from UID + event type if not provided
		idempotencyKey = fmt.Sprintf("%s-%s", paymentUID, eventType)
	}

	var count int
	query := `
		SELECT COUNT(*) FROM payment_audits 
		WHERE payment_uid = $1 
		AND event_type = $2 
		AND idempotency_key = $3
		AND is_duplicate = FALSE`

	err := r.db.GetContext(ctx, &count, query, paymentUID, eventType, idempotencyKey)
	if err != nil {
		return false, fmt.Errorf("failed to check duplicate: %w", err)
	}

	return count > 0, nil
}

// GetByPaymentUID retrieves all audit entries for a payment UID
func (r *PaymentAuditRepository) GetByPaymentUID(ctx context.Context, paymentUID string) ([]*models.PaymentAudit, error) {
	var audits []*models.PaymentAudit
	query := `
		SELECT * FROM payment_audits 
		WHERE payment_uid = $1 
		ORDER BY created_at ASC`

	err := r.db.SelectContext(ctx, &audits, query, paymentUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by payment UID: %w", err)
	}

	return audits, nil
}

// GetByIntentID retrieves all audit entries for an intent
func (r *PaymentAuditRepository) GetByIntentID(ctx context.Context, intentID uuid.UUID) ([]*models.PaymentAudit, error) {
	var audits []*models.PaymentAudit
	query := `
		SELECT * FROM payment_audits 
		WHERE intent_id = $1 
		ORDER BY created_at ASC`

	err := r.db.SelectContext(ctx, &audits, query, intentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by intent ID: %w", err)
	}

	return audits, nil
}

// GetAmountMismatches retrieves all audits where amounts don't match
// This is CRITICAL for fraud detection
func (r *PaymentAuditRepository) GetAmountMismatches(ctx context.Context, limit int) ([]*models.PaymentAudit, error) {
	var audits []*models.PaymentAudit
	query := `
		SELECT * FROM payment_audits 
		WHERE amounts_match = FALSE 
		ORDER BY created_at DESC 
		LIMIT $1`

	err := r.db.SelectContext(ctx, &audits, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get amount mismatches: %w", err)
	}

	return audits, nil
}

// GetRecentByEventType retrieves recent events of a specific type
func (r *PaymentAuditRepository) GetRecentByEventType(ctx context.Context, eventType models.PaymentEventType, hours int, limit int) ([]*models.PaymentAudit, error) {
	var audits []*models.PaymentAudit
	query := `
		SELECT * FROM payment_audits 
		WHERE event_type = $1 
		AND created_at > NOW() - INTERVAL '1 hour' * $2
		ORDER BY created_at DESC 
		LIMIT $3`

	err := r.db.SelectContext(ctx, &audits, query, eventType, hours, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent events: %w", err)
	}

	return audits, nil
}
