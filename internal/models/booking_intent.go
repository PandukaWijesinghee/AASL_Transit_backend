package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// BOOKING INTENT TYPES & STATUSES (matches DB ENUMs)
// ============================================================================

// BookingIntentStatus represents the status of a booking intent
// Matches PostgreSQL ENUM: booking_intent_status
type BookingIntentStatus string

const (
	IntentStatusHeld               BookingIntentStatus = "held"                // Seats/lounges locked, waiting for payment
	IntentStatusPaymentPending     BookingIntentStatus = "payment_pending"     // Payment initiated
	IntentStatusConfirming         BookingIntentStatus = "confirming"          // Processing confirmation
	IntentStatusConfirmed          BookingIntentStatus = "confirmed"           // Payment success, bookings created
	IntentStatusExpired            BookingIntentStatus = "expired"             // TTL expired, holds released
	IntentStatusCancelled          BookingIntentStatus = "cancelled"           // User cancelled
	IntentStatusConfirmationFailed BookingIntentStatus = "confirmation_failed" // Confirm failed after payment (needs refund)
	IntentStatusRefundInitiated    BookingIntentStatus = "refund_initiated"    // Refund in progress
	IntentStatusRefunded           BookingIntentStatus = "refunded"            // Refund completed
)

// IntentPaymentStatus represents the payment status within an intent
// Matches PostgreSQL ENUM: intent_payment_status
type IntentPaymentStatus string

const (
	IntentPaymentPending    IntentPaymentStatus = "pending"
	IntentPaymentProcessing IntentPaymentStatus = "processing"
	IntentPaymentSuccess    IntentPaymentStatus = "success"
	IntentPaymentFailed     IntentPaymentStatus = "failed"
	IntentPaymentRefunded   IntentPaymentStatus = "refunded"
)

// BookingIntentType represents the type of booking intent
type BookingIntentType string

const (
	IntentTypeBusOnly    BookingIntentType = "bus_only"
	IntentTypeLoungeOnly BookingIntentType = "lounge_only"
	IntentTypeCombined   BookingIntentType = "combined"
)

// ============================================================================
// JSONB PAYLOAD TYPES
// ============================================================================

// BusIntentPayload stores bus booking intent data in JSONB
type BusIntentPayload struct {
	ScheduledTripID   string             `json:"scheduled_trip_id"`
	BoardingStopID    *string            `json:"boarding_stop_id,omitempty"`
	BoardingStopName  string             `json:"boarding_stop_name"`
	AlightingStopID   *string            `json:"alighting_stop_id,omitempty"`
	AlightingStopName string             `json:"alighting_stop_name"`
	Seats             []BusIntentSeat    `json:"seats"`
	PassengerName     string             `json:"passenger_name"`
	PassengerPhone    string             `json:"passenger_phone"`
	PassengerEmail    *string            `json:"passenger_email,omitempty"`
	SpecialRequests   *string            `json:"special_requests,omitempty"`
	TripInfo          *BusIntentTripInfo `json:"trip_info,omitempty"` // Denormalized for display
}

// BusIntentSeat represents a seat selection in bus intent
type BusIntentSeat struct {
	TripSeatID      string  `json:"trip_seat_id"`
	SeatNumber      string  `json:"seat_number"`
	SeatType        string  `json:"seat_type,omitempty"`
	SeatPrice       float64 `json:"seat_price"`
	PassengerName   string  `json:"passenger_name"`
	PassengerPhone  *string `json:"passenger_phone,omitempty"`
	PassengerGender *string `json:"passenger_gender,omitempty"`
	IsPrimary       bool    `json:"is_primary"`
}

// BusIntentTripInfo stores trip details for display (denormalized snapshot)
type BusIntentTripInfo struct {
	RouteName         string    `json:"route_name"`
	BusNumber         string    `json:"bus_number,omitempty"`
	BusType           string    `json:"bus_type,omitempty"`
	DepartureDatetime time.Time `json:"departure_datetime"`
}

// LoungeIntentPayload stores lounge booking intent data in JSONB
type LoungeIntentPayload struct {
	LoungeID      string                 `json:"lounge_id"`
	LoungeName    string                 `json:"lounge_name"`
	LoungeAddress *string                `json:"lounge_address,omitempty"`
	PricingType   string                 `json:"pricing_type"`  // "1_hour", "2_hours", "3_hours", "until_bus"
	Date          string                 `json:"date"`          // "2025-12-15"
	CheckInTime   string                 `json:"check_in_time"` // "09:00"
	CheckOutTime  *string                `json:"check_out_time,omitempty"`
	GuestCount    int                    `json:"guest_count"` // Total: primary + additional guests
	Guests        []LoungeIntentGuest    `json:"guests"`
	PreOrders     []LoungeIntentPreOrder `json:"pre_orders,omitempty"`
	PricePerGuest float64                `json:"price_per_guest"`
	BasePrice     float64                `json:"base_price"` // price_per_guest * guest_count
	PreOrderTotal float64                `json:"pre_order_total"`
	TotalPrice    float64                `json:"total_price"` // base_price + pre_order_total
}

// LoungeIntentGuest represents a guest in lounge intent
type LoungeIntentGuest struct {
	GuestName  string  `json:"guest_name"`
	GuestPhone *string `json:"guest_phone,omitempty"`
	IsPrimary  bool    `json:"is_primary"`
}

// LoungeIntentPreOrder represents a pre-ordered item
type LoungeIntentPreOrder struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductType string  `json:"product_type,omitempty"`
	ImageURL    *string `json:"image_url,omitempty"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
}

// PricingSnapshot stores server-calculated prices at intent creation
type PricingSnapshot struct {
	BusFare         float64             `json:"bus_fare"`
	PreLoungeFare   float64             `json:"pre_lounge_fare"`
	PostLoungeFare  float64             `json:"post_lounge_fare"`
	Total           float64             `json:"total"`
	Currency        string              `json:"currency"`
	CalculatedAt    time.Time           `json:"calculated_at"`
	SeatPrices      map[string]float64  `json:"seat_prices,omitempty"` // seat_id -> price
	DiscountApplied *IntentDiscountInfo `json:"discount_applied,omitempty"`
}

// IntentDiscountInfo stores discount information
type IntentDiscountInfo struct {
	Code           string  `json:"code"`
	DiscountType   string  `json:"discount_type"` // "percentage" or "fixed"
	DiscountValue  float64 `json:"discount_value"`
	DiscountAmount float64 `json:"discount_amount"` // Actual amount discounted
}

// ============================================================================
// JSONB SCANNER/VALUER IMPLEMENTATIONS
// ============================================================================

func (p BusIntentPayload) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *BusIntentPayload) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for BusIntentPayload")
	}
	return json.Unmarshal(bytes, p)
}

func (p LoungeIntentPayload) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *LoungeIntentPayload) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for LoungeIntentPayload")
	}
	return json.Unmarshal(bytes, p)
}

func (p PricingSnapshot) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *PricingSnapshot) Scan(value interface{}) error {
	if value == nil {
		*p = PricingSnapshot{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for PricingSnapshot")
	}
	return json.Unmarshal(bytes, p)
}

// ============================================================================
// BOOKING INTENT MODEL (booking_intents table)
// ============================================================================

// BookingIntent represents a booking intent before payment confirmation
type BookingIntent struct {
	ID     uuid.UUID `json:"id" db:"id"`
	UserID uuid.UUID `json:"user_id" db:"user_id"`

	// Intent type
	IntentType BookingIntentType   `json:"intent_type" db:"intent_type"`
	Status     BookingIntentStatus `json:"status" db:"status"`

	// JSONB payloads (nullable in DB)
	BusIntent            *BusIntentPayload    `json:"bus_intent,omitempty" db:"bus_intent"`
	PreTripLoungeIntent  *LoungeIntentPayload `json:"pre_trip_lounge_intent,omitempty" db:"pre_trip_lounge_intent"`
	PostTripLoungeIntent *LoungeIntentPayload `json:"post_trip_lounge_intent,omitempty" db:"post_trip_lounge_intent"`

	// Pricing (server-calculated, stored at intent time)
	BusFare         float64         `json:"bus_fare" db:"bus_fare"`
	PreLoungeFare   float64         `json:"pre_lounge_fare" db:"pre_lounge_fare"`
	PostLoungeFare  float64         `json:"post_lounge_fare" db:"post_lounge_fare"`
	TotalAmount     float64         `json:"total_amount" db:"total_amount"`
	Currency        string          `json:"currency" db:"currency"`
	PricingSnapshot PricingSnapshot `json:"pricing_snapshot" db:"pricing_snapshot"`

	// Payment tracking
	PaymentReference       *string              `json:"payment_reference,omitempty" db:"payment_reference"`
	PaymentStatus          *IntentPaymentStatus `json:"payment_status,omitempty" db:"payment_status"`
	PaymentGateway         string               `json:"payment_gateway" db:"payment_gateway"`
	PaymentUID             *string              `json:"payment_uid,omitempty" db:"payment_uid"`                           // PAYable unique transaction ID
	PaymentStatusIndicator *string              `json:"payment_status_indicator,omitempty" db:"payment_status_indicator"` // PAYable status check token

	// Passenger info (extracted from bus_intent for convenience)
	PassengerName  string `json:"passenger_name,omitempty" db:"passenger_name"`
	PassengerPhone string `json:"passenger_phone,omitempty" db:"passenger_phone"`

	// Result references (filled AFTER confirmation)
	BusBookingID        *uuid.UUID `json:"bus_booking_id,omitempty" db:"bus_booking_id"`
	PreLoungeBookingID  *uuid.UUID `json:"pre_lounge_booking_id,omitempty" db:"pre_lounge_booking_id"`
	PostLoungeBookingID *uuid.UUID `json:"post_lounge_booking_id,omitempty" db:"post_lounge_booking_id"`

	// TTL Management
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`

	// Timestamps
	PaymentInitiatedAt *time.Time `json:"payment_initiated_at,omitempty" db:"payment_initiated_at"`
	ConfirmedAt        *time.Time `json:"confirmed_at,omitempty" db:"confirmed_at"`
	ExpiredAt          *time.Time `json:"expired_at,omitempty" db:"expired_at"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`

	// Idempotency
	IdempotencyKey *string `json:"idempotency_key,omitempty" db:"idempotency_key"`
}

// IsExpired checks if the intent has passed its TTL
func (i *BookingIntent) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// CanInitiatePayment checks if payment can be initiated
// Allows both 'held' (first time) and 'payment_pending' (retry)
func (i *BookingIntent) CanInitiatePayment() bool {
	return (i.Status == IntentStatusHeld || i.Status == IntentStatusPaymentPending) && !i.IsExpired()
}

// CanConfirm checks if the intent can be confirmed
func (i *BookingIntent) CanConfirm() bool {
	return (i.Status == IntentStatusPaymentPending || i.Status == IntentStatusHeld) && !i.IsExpired()
}

// ============================================================================
// LOUNGE CAPACITY HOLD MODEL (lounge_capacity_holds table)
// ============================================================================

// LoungeCapacityHold tracks lounge capacity holds during booking intent TTL
type LoungeCapacityHold struct {
	ID            uuid.UUID `json:"id" db:"id"`
	LoungeID      uuid.UUID `json:"lounge_id" db:"lounge_id"`
	IntentID      uuid.UUID `json:"intent_id" db:"intent_id"`
	Date          time.Time `json:"date" db:"date"`
	TimeSlotStart string    `json:"time_slot_start" db:"time_slot_start"` // TIME as string "09:00"
	TimeSlotEnd   string    `json:"time_slot_end" db:"time_slot_end"`
	GuestsCount   int       `json:"guests_count" db:"guests_count"`
	HeldUntil     time.Time `json:"held_until" db:"held_until"`
	Status        string    `json:"status" db:"status"` // "held", "confirmed", "released"
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ============================================================================
// REQUEST/RESPONSE STRUCTS
// ============================================================================

// CreateBookingIntentRequest is the request to create a booking intent
type CreateBookingIntentRequest struct {
	IntentType BookingIntentType `json:"intent_type" binding:"required"`

	// Bus booking data (required for bus_only and combined)
	Bus *BusIntentRequest `json:"bus,omitempty"`

	// Lounge booking data (optional)
	PreTripLounge  *LoungeIntentRequest `json:"pre_trip_lounge,omitempty"`
	PostTripLounge *LoungeIntentRequest `json:"post_trip_lounge,omitempty"`

	// Idempotency key (optional)
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

// BusIntentRequest represents bus booking request data
type BusIntentRequest struct {
	ScheduledTripID   string                 `json:"scheduled_trip_id" binding:"required"`
	BoardingStopID    *string                `json:"boarding_stop_id,omitempty"`
	BoardingStopName  string                 `json:"boarding_stop_name" binding:"required"`
	AlightingStopID   *string                `json:"alighting_stop_id,omitempty"`
	AlightingStopName string                 `json:"alighting_stop_name" binding:"required"`
	Seats             []BusIntentSeatRequest `json:"seats" binding:"required,min=1"`
	PassengerName     string                 `json:"passenger_name" binding:"required"`
	PassengerPhone    string                 `json:"passenger_phone" binding:"required"`
	PassengerEmail    *string                `json:"passenger_email,omitempty"`
	SpecialRequests   *string                `json:"special_requests,omitempty"`
}

// BusIntentSeatRequest represents a seat in the request
type BusIntentSeatRequest struct {
	TripSeatID      string  `json:"trip_seat_id" binding:"required"`
	SeatNumber      string  `json:"seat_number" binding:"required"`
	PassengerName   string  `json:"passenger_name" binding:"required"`
	PassengerPhone  *string `json:"passenger_phone,omitempty"`
	PassengerGender *string `json:"passenger_gender,omitempty"`
	IsPrimary       bool    `json:"is_primary"`
}

// LoungeIntentRequest represents lounge booking request data
type LoungeIntentRequest struct {
	LoungeID    string                        `json:"lounge_id" binding:"required"`
	PricingType string                        `json:"pricing_type" binding:"required"` // "1_hour", "2_hours", "3_hours", "until_bus"
	Guests      []LoungeIntentGuestRequest    `json:"guests" binding:"required,min=1"`
	PreOrders   []LoungeIntentPreOrderRequest `json:"pre_orders,omitempty"`
}

// LoungeIntentGuestRequest represents a guest in the request
type LoungeIntentGuestRequest struct {
	GuestName  string  `json:"guest_name" binding:"required"`
	GuestPhone *string `json:"guest_phone,omitempty"`
}

// LoungeIntentPreOrderRequest represents a pre-order in the request
type LoungeIntentPreOrderRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

// Validate validates the booking intent request
func (r *CreateBookingIntentRequest) Validate() error {
	switch r.IntentType {
	case IntentTypeBusOnly:
		if r.Bus == nil {
			return errors.New("bus data is required for bus_only intent")
		}
		if r.PreTripLounge != nil || r.PostTripLounge != nil {
			return errors.New("lounge data should not be present for bus_only intent")
		}
	case IntentTypeLoungeOnly:
		if r.Bus != nil {
			return errors.New("bus data should not be present for lounge_only intent")
		}
		if r.PreTripLounge == nil && r.PostTripLounge == nil {
			return errors.New("at least one lounge booking is required for lounge_only intent")
		}
	case IntentTypeCombined:
		if r.Bus == nil {
			return errors.New("bus data is required for combined intent")
		}
		if r.PreTripLounge == nil && r.PostTripLounge == nil {
			return errors.New("at least one lounge booking is required for combined intent")
		}
	default:
		return errors.New("invalid intent_type: must be bus_only, lounge_only, or combined")
	}

	// Validate bus seats
	if r.Bus != nil {
		if len(r.Bus.Seats) == 0 {
			return errors.New("at least one seat must be selected")
		}
		if len(r.Bus.Seats) > 10 {
			return errors.New("maximum 10 seats can be booked at once")
		}
		// Ensure at least one primary seat
		hasPrimary := false
		for _, seat := range r.Bus.Seats {
			if seat.IsPrimary {
				hasPrimary = true
				break
			}
		}
		if !hasPrimary {
			r.Bus.Seats[0].IsPrimary = true
		}
	}

	// Validate lounge guests
	if r.PreTripLounge != nil && len(r.PreTripLounge.Guests) == 0 {
		return errors.New("at least one guest is required for pre-trip lounge")
	}
	if r.PostTripLounge != nil && len(r.PostTripLounge.Guests) == 0 {
		return errors.New("at least one guest is required for post-trip lounge")
	}

	return nil
}

// BookingIntentResponse is the response after creating an intent
type BookingIntentResponse struct {
	IntentID       uuid.UUID      `json:"intent_id"`
	Status         string         `json:"status"`
	PriceBreakdown PriceBreakdown `json:"price_breakdown"`
	ExpiresAt      time.Time      `json:"expires_at"`
	TTLSeconds     int            `json:"ttl_seconds"` // Remaining TTL for countdown

	// Availability status
	SeatAvailabilityChecked   bool `json:"seat_availability_checked"`
	LoungeAvailabilityChecked bool `json:"lounge_availability_checked"`
}

// PriceBreakdown shows pricing details
type PriceBreakdown struct {
	BusFare        float64 `json:"bus_fare"`
	PreLoungeFare  float64 `json:"pre_lounge_fare"`
	PostLoungeFare float64 `json:"post_lounge_fare"`
	Total          float64 `json:"total"`
	Currency       string  `json:"currency"`
}

// InitiatePaymentResponse is returned when initiating payment
type InitiatePaymentResponse struct {
	PaymentURL      string    `json:"payment_url"`
	InvoiceID       string    `json:"invoice_id"`
	Amount          string    `json:"amount"`
	Currency        string    `json:"currency"`
	UID             string    `json:"uid,omitempty"`              // PAYable unique transaction ID
	StatusIndicator string    `json:"status_indicator,omitempty"` // PAYable status check token
	ExpiresAt       time.Time `json:"expires_at"`
}

// ConfirmBookingRequest is the request to confirm a booking after payment
type ConfirmBookingRequest struct {
	IntentID         string  `json:"intent_id" binding:"required"`
	PaymentReference *string `json:"payment_reference,omitempty"` // Optional if webhook already set it
}

// ConfirmBookingResponse is returned after successful confirmation
type ConfirmBookingResponse struct {
	MasterReference string `json:"master_reference"` // Overall booking reference

	BusBooking        *ConfirmedBusBooking    `json:"bus_booking,omitempty"`
	PreLoungeBooking  *ConfirmedLoungeBooking `json:"pre_lounge_booking,omitempty"`
	PostLoungeBooking *ConfirmedLoungeBooking `json:"post_lounge_booking,omitempty"`

	TotalPaid float64 `json:"total_paid"`
	Currency  string  `json:"currency"`
}

// ConfirmedBusBooking represents the confirmed bus booking details
type ConfirmedBusBooking struct {
	ID        uuid.UUID `json:"id"`
	Reference string    `json:"reference"`
	QRCode    string    `json:"qr_code"` // Base64 encoded QR or QR data string
}

// ConfirmedLoungeBooking represents the confirmed lounge booking details
type ConfirmedLoungeBooking struct {
	ID        uuid.UUID `json:"id"`
	Reference string    `json:"reference"`
	QRCode    *string   `json:"qr_code,omitempty"`
}

// GetIntentStatusResponse is the response for getting intent status
type GetIntentStatusResponse struct {
	IntentID       uuid.UUID            `json:"intent_id"`
	Status         BookingIntentStatus  `json:"status"`
	PaymentStatus  *IntentPaymentStatus `json:"payment_status,omitempty"`
	PriceBreakdown PriceBreakdown       `json:"price_breakdown"`
	ExpiresAt      time.Time            `json:"expires_at"`
	IsExpired      bool                 `json:"is_expired"`

	// Booking results (if confirmed)
	Bookings *ConfirmBookingResponse `json:"bookings,omitempty"`
}

// ============================================================================
// PARTIAL AVAILABILITY ERROR
// ============================================================================

// PartialAvailabilityError is returned when some items are unavailable
type PartialAvailabilityError struct {
	Available   AvailabilityStatus `json:"available"`
	Unavailable UnavailableItems   `json:"unavailable"`
	Message     string             `json:"message"`
}

// AvailabilityStatus shows what is available
type AvailabilityStatus struct {
	Bus        *ItemAvailability `json:"bus,omitempty"`
	PreLounge  *ItemAvailability `json:"pre_lounge,omitempty"`
	PostLounge *ItemAvailability `json:"post_lounge,omitempty"`
}

// ItemAvailability represents availability of an item
type ItemAvailability struct {
	Status string  `json:"status"` // "can_hold", "unavailable"
	Fare   float64 `json:"fare,omitempty"`
}

// UnavailableItems shows what is not available
type UnavailableItems struct {
	Bus        *UnavailableReason `json:"bus,omitempty"`
	PreLounge  *UnavailableReason `json:"pre_lounge,omitempty"`
	PostLounge *UnavailableReason `json:"post_lounge,omitempty"`
}

// UnavailableReason explains why something is unavailable
type UnavailableReason struct {
	Reason       string        `json:"reason"` // "seats_taken", "fully_booked", "trip_departed"
	Details      string        `json:"details,omitempty"`
	TakenSeats   []string      `json:"taken_seats,omitempty"` // Which specific seats are taken
	Alternatives []Alternative `json:"alternatives,omitempty"`
}

// Alternative suggests alternatives when something is unavailable
type Alternative struct {
	Type        string  `json:"type"` // "seat", "lounge", "trip"
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price,omitempty"`
}

func (e *PartialAvailabilityError) Error() string {
	return e.Message
}
