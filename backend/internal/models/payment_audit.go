package models

import (
	"time"

	"github.com/google/uuid"
)

// PaymentEventType represents the type of payment event
type PaymentEventType string

const (
	PaymentEventInitiated              PaymentEventType = "payment_initiated"
	PaymentEventResponse               PaymentEventType = "payment_response"
	PaymentEventWebhookReceived        PaymentEventType = "webhook_received"
	PaymentEventStatusCheckRequest     PaymentEventType = "status_check_request"
	PaymentEventStatusCheckResponse    PaymentEventType = "status_check_response"
	PaymentEventSuccess                PaymentEventType = "payment_success"
	PaymentEventFailed                 PaymentEventType = "payment_failed"
	PaymentEventCancelled              PaymentEventType = "payment_cancelled"
	PaymentEventBookingConfirmed       PaymentEventType = "booking_confirmed"
	PaymentEventBookingConfirmFailed   PaymentEventType = "booking_confirmation_failed"
	PaymentEventRefundInitiated        PaymentEventType = "refund_initiated"
	PaymentEventRefundCompleted        PaymentEventType = "refund_completed"
	PaymentEventPartialRefund          PaymentEventType = "partial_refund"
	PaymentEventChargebackReceived     PaymentEventType = "chargeback_received"
	PaymentEventChargebackWon          PaymentEventType = "chargeback_won"
	PaymentEventChargebackLost         PaymentEventType = "chargeback_lost"
	PaymentEventReconciliationMismatch PaymentEventType = "reconciliation_mismatch"
	PaymentEventError                  PaymentEventType = "error"
)

// PaymentEventSource identifies where the event originated
type PaymentEventSource string

const (
	PaymentSourceBackend        PaymentEventSource = "backend"
	PaymentSourcePayableWebhook PaymentEventSource = "payable_webhook"
	PaymentSourcePayableAPI     PaymentEventSource = "payable_api"
	PaymentSourceUser           PaymentEventSource = "user"
	PaymentSourceSystem         PaymentEventSource = "system"
)

// PaymentAudit represents an immutable audit log entry for payment events
type PaymentAudit struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	IntentID         *uuid.UUID `json:"intent_id,omitempty" db:"intent_id"`
	PaymentUID       *string    `json:"payment_uid,omitempty" db:"payment_uid"`
	PaymentReference *string    `json:"payment_reference,omitempty" db:"payment_reference"`

	// Event info
	EventType   PaymentEventType   `json:"event_type" db:"event_type"`
	EventSource PaymentEventSource `json:"event_source" db:"event_source"`

	// Amount tracking - CRITICAL for verification
	ExpectedAmount *float64 `json:"expected_amount,omitempty" db:"expected_amount"`
	ReceivedAmount *float64 `json:"received_amount,omitempty" db:"received_amount"`
	Currency       *string  `json:"currency,omitempty" db:"currency"`
	AmountsMatch   *bool    `json:"amounts_match,omitempty" db:"amounts_match"`

	// Status
	PaymentStatus        *string `json:"payment_status,omitempty" db:"payment_status"`
	GatewayTransactionID *string `json:"gateway_transaction_id,omitempty" db:"gateway_transaction_id"`

	// Raw payloads - CRITICAL for debugging
	RequestPayload  JSONB   `json:"request_payload,omitempty" db:"request_payload"`
	ResponsePayload JSONB   `json:"response_payload,omitempty" db:"response_payload"`
	RawBody         *string `json:"raw_body,omitempty" db:"raw_body"`

	// HTTP details
	HTTPStatusCode *int    `json:"http_status_code,omitempty" db:"http_status_code"`
	HTTPMethod     *string `json:"http_method,omitempty" db:"http_method"`
	EndpointURL    *string `json:"endpoint_url,omitempty" db:"endpoint_url"`

	// Error tracking
	ErrorMessage *string `json:"error_message,omitempty" db:"error_message"`
	ErrorCode    *string `json:"error_code,omitempty" db:"error_code"`

	// Processing info
	ProcessingTimeMs *int    `json:"processing_time_ms,omitempty" db:"processing_time_ms"`
	IsDuplicate      bool    `json:"is_duplicate" db:"is_duplicate"`
	IdempotencyKey   *string `json:"idempotency_key,omitempty" db:"idempotency_key"`

	// Metadata
	IPAddress     *string `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent     *string `json:"user_agent,omitempty" db:"user_agent"`
	CorrelationID *string `json:"correlation_id,omitempty" db:"correlation_id"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty" db:"processed_at"`
}

// Note: JSONB type is defined in bus_owner.go and reused here

// NewPaymentAudit creates a new payment audit entry with required fields
func NewPaymentAudit(eventType PaymentEventType, source PaymentEventSource) *PaymentAudit {
	return &PaymentAudit{
		ID:          uuid.New(),
		EventType:   eventType,
		EventSource: source,
		CreatedAt:   time.Now(),
		IsDuplicate: false,
	}
}

// SetIntent sets the intent ID for the audit
func (pa *PaymentAudit) SetIntent(intentID uuid.UUID) *PaymentAudit {
	pa.IntentID = &intentID
	return pa
}

// SetPaymentUID sets the PAYable UID
func (pa *PaymentAudit) SetPaymentUID(uid string) *PaymentAudit {
	pa.PaymentUID = &uid
	return pa
}

// SetPaymentReference sets our invoice ID
func (pa *PaymentAudit) SetPaymentReference(ref string) *PaymentAudit {
	pa.PaymentReference = &ref
	return pa
}

// SetAmounts sets and verifies amounts - returns whether they match
func (pa *PaymentAudit) SetAmounts(expected, received float64, currency string) bool {
	pa.ExpectedAmount = &expected
	pa.ReceivedAmount = &received
	pa.Currency = &currency

	// Compare with tolerance for floating point
	const tolerance = 0.01
	match := abs(expected-received) < tolerance
	pa.AmountsMatch = &match
	return match
}

// SetPaymentStatus sets the payment status from gateway
func (pa *PaymentAudit) SetPaymentStatus(status string) *PaymentAudit {
	pa.PaymentStatus = &status
	return pa
}

// SetError sets error information
func (pa *PaymentAudit) SetError(message string, code *string) *PaymentAudit {
	pa.ErrorMessage = &message
	pa.ErrorCode = code
	return pa
}

// SetRawBody stores the raw response body before parsing
func (pa *PaymentAudit) SetRawBody(body string) *PaymentAudit {
	pa.RawBody = &body
	return pa
}

// SetHTTPDetails sets HTTP request/response details
func (pa *PaymentAudit) SetHTTPDetails(method string, url string, statusCode int) *PaymentAudit {
	pa.HTTPMethod = &method
	pa.EndpointURL = &url
	pa.HTTPStatusCode = &statusCode
	return pa
}

// SetRequestPayload sets the request payload sent
func (pa *PaymentAudit) SetRequestPayload(payload map[string]interface{}) *PaymentAudit {
	pa.RequestPayload = JSONB(payload)
	return pa
}

// SetResponsePayload sets the response payload received
func (pa *PaymentAudit) SetResponsePayload(payload map[string]interface{}) *PaymentAudit {
	pa.ResponsePayload = JSONB(payload)
	return pa
}

// SetMetadata sets request metadata
func (pa *PaymentAudit) SetMetadata(ip, userAgent, correlationID string) *PaymentAudit {
	if ip != "" {
		pa.IPAddress = &ip
	}
	if userAgent != "" {
		pa.UserAgent = &userAgent
	}
	if correlationID != "" {
		pa.CorrelationID = &correlationID
	}
	return pa
}

// SetProcessingTime calculates and sets processing time
func (pa *PaymentAudit) SetProcessingTime(startTime time.Time) *PaymentAudit {
	durationMs := int(time.Since(startTime).Milliseconds())
	pa.ProcessingTimeMs = &durationMs
	now := time.Now()
	pa.ProcessedAt = &now
	return pa
}

// MarkAsDuplicate marks this event as a duplicate
func (pa *PaymentAudit) MarkAsDuplicate() *PaymentAudit {
	pa.IsDuplicate = true
	return pa
}

// SetIdempotencyKey sets the idempotency key
func (pa *PaymentAudit) SetIdempotencyKey(key string) *PaymentAudit {
	pa.IdempotencyKey = &key
	return pa
}

// abs returns absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
