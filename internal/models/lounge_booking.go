package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// LOUNGE BOOKING TYPES & STATUSES
// ============================================================================

// LoungeBookingType represents the type of lounge booking
type LoungeBookingType string

const (
	LoungeBookingPreTrip    LoungeBookingType = "pre_trip"   // Before bus departure
	LoungeBookingPostTrip   LoungeBookingType = "post_trip"  // After bus arrival
	LoungeBookingStandalone LoungeBookingType = "standalone" // Independent lounge visit
)

// LoungeBookingLoungeType represents the lounge type ENUM
type LoungeBookingLoungeType string

const (
	LoungeBookingLoungeTypeHourly  LoungeBookingLoungeType = "hourly"
	LoungeBookingLoungeTypeDayPass LoungeBookingLoungeType = "day_pass"
	LoungeBookingLoungeTypePremium LoungeBookingLoungeType = "premium"
)

// LoungeBookingDurationType represents the duration type ENUM
type LoungeBookingDurationType string

const (
	LoungeBookingDurationTypeHours   LoungeBookingDurationType = "hours"
	LoungeBookingDurationTypeDays    LoungeBookingDurationType = "days"
	LoungeBookingDurationTypeMinutes LoungeBookingDurationType = "minutes"
)

// LoungeBookingStatus represents the status of a lounge booking
type LoungeBookingStatus string

const (
	LoungeBookingStatusPending    LoungeBookingStatus = "pending"
	LoungeBookingStatusConfirmed  LoungeBookingStatus = "confirmed"
	LoungeBookingStatusCheckedIn  LoungeBookingStatus = "checked_in"
	LoungeBookingStatusInLounge   LoungeBookingStatus = "in_lounge"
	LoungeBookingStatusCheckedOut LoungeBookingStatus = "checked_out"
	LoungeBookingStatusCompleted  LoungeBookingStatus = "completed"
	LoungeBookingStatusCancelled  LoungeBookingStatus = "cancelled"
	LoungeBookingStatusNoShow     LoungeBookingStatus = "no_show"
)

// LoungeBookingPreOrderStatus represents the pre-order status ENUM
type LoungeBookingPreOrderStatus string

const (
	LoungeBookingPreOrderStatusPending   LoungeBookingPreOrderStatus = "pending"
	LoungeBookingPreOrderStatusConfirmed LoungeBookingPreOrderStatus = "confirmed"
	LoungeBookingPreOrderStatusPreparing LoungeBookingPreOrderStatus = "preparing"
	LoungeBookingPreOrderStatusReady     LoungeBookingPreOrderStatus = "ready"
	LoungeBookingPreOrderStatusDelivered LoungeBookingPreOrderStatus = "delivered"
	LoungeBookingPreOrderStatusCancelled LoungeBookingPreOrderStatus = "cancelled"
)

// LoungeBookingDeliveryPreference represents the delivery preference ENUM
type LoungeBookingDeliveryPreference string

const (
	LoungeBookingDeliveryPreferenceOnArrival LoungeBookingDeliveryPreference = "on_arrival"
	LoungeBookingDeliveryPreferenceScheduled LoungeBookingDeliveryPreference = "scheduled"
	LoungeBookingDeliveryPreferenceImmediate LoungeBookingDeliveryPreference = "immediate"
)

// LoungeOrderStatus represents the status of an in-lounge order
type LoungeOrderStatus string

const (
	LoungeOrderStatusPending   LoungeOrderStatus = "pending"
	LoungeOrderStatusConfirmed LoungeOrderStatus = "confirmed"
	LoungeOrderStatusPreparing LoungeOrderStatus = "preparing"
	LoungeOrderStatusReady     LoungeOrderStatus = "ready"
	LoungeOrderStatusServed    LoungeOrderStatus = "served"
	LoungeOrderStatusCompleted LoungeOrderStatus = "completed"
	LoungeOrderStatusCancelled LoungeOrderStatus = "cancelled"
)

// LoungeOrderPaymentStatus represents payment status ENUM for lounge orders
type LoungeOrderPaymentStatus string

const (
	LoungeOrderPaymentStatusPending  LoungeOrderPaymentStatus = "pending"
	LoungeOrderPaymentStatusPaid     LoungeOrderPaymentStatus = "paid"
	LoungeOrderPaymentStatusFailed   LoungeOrderPaymentStatus = "failed"
	LoungeOrderPaymentStatusRefunded LoungeOrderPaymentStatus = "refunded"
	LoungeOrderPaymentStatusPartial  LoungeOrderPaymentStatus = "partial"
)

// LoungeOrderItemStatus represents the order item status ENUM
type LoungeOrderItemStatus string

const (
	LoungeOrderItemStatusPending   LoungeOrderItemStatus = "pending"
	LoungeOrderItemStatusConfirmed LoungeOrderItemStatus = "confirmed"
	LoungeOrderItemStatusPreparing LoungeOrderItemStatus = "preparing"
	LoungeOrderItemStatusReady     LoungeOrderItemStatus = "ready"
	LoungeOrderItemStatusServed    LoungeOrderItemStatus = "served"
	LoungeOrderItemStatusCancelled LoungeOrderItemStatus = "cancelled"
)

// LoungePaymentStatus represents payment status for lounge services (legacy, use LoungeOrderPaymentStatus)
type LoungePaymentStatus string

const (
	LoungePaymentPending       LoungePaymentStatus = "pending"
	LoungePaymentPaid          LoungePaymentStatus = "paid"
	LoungePaymentPartial       LoungePaymentStatus = "partial"
	LoungePaymentCollectOnSite LoungePaymentStatus = "collect_on_site"
	LoungePaymentFailed        LoungePaymentStatus = "failed"
	LoungePaymentRefunded      LoungePaymentStatus = "refunded"
)

// ============================================================================
// LOUNGE MARKETPLACE CATEGORY (lounge_marketplace_categories table)
// ============================================================================

// LoungeMarketplaceCategory represents a product category
type LoungeMarketplaceCategory struct {
	ID           uuid.UUID      `db:"id" json:"id"`
	Name         string         `db:"name" json:"name"`
	Description  sql.NullString `db:"description" json:"description,omitempty"`
	IconURL      sql.NullString `db:"icon_url" json:"icon_url,omitempty"`
	DisplayOrder int            `db:"display_order" json:"display_order"`
	IsActive     bool           `db:"is_active" json:"is_active"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// LOUNGE PRODUCT (lounge_products table)
// ============================================================================

// LoungeProductStockStatus represents the stock status ENUM
type LoungeProductStockStatus string

const (
	LoungeProductStockStatusInStock      LoungeProductStockStatus = "in_stock"
	LoungeProductStockStatusLowStock     LoungeProductStockStatus = "low_stock"
	LoungeProductStockStatusOutOfStock   LoungeProductStockStatus = "out_of_stock"
	LoungeProductStockStatusDiscontinued LoungeProductStockStatus = "discontinued"
	LoungeProductStockStatusMadeToOrder  LoungeProductStockStatus = "made_to_order" // Fresh-cooked items
)

// LoungeProductType represents the product type ENUM
type LoungeProductType string

const (
	LoungeProductTypeProduct LoungeProductType = "product" // Food, beverages, snacks, etc.
	LoungeProductTypeService LoungeProductType = "service" // Storage, WiFi, shower, etc.
	LoungeProductTypePackage LoungeProductType = "package" // Bundled offerings
)

// LoungeProduct represents a product/service offered by a lounge
type LoungeProduct struct {
	ID                     uuid.UUID                `db:"id" json:"id"`
	LoungeID               uuid.UUID                `db:"lounge_id" json:"lounge_id"`
	CategoryID             uuid.UUID                `db:"category_id" json:"category_id"`
	Name                   string                   `db:"name" json:"name"`
	Description            *string                  `db:"description" json:"description,omitempty"`
	Price                  string                   `db:"price" json:"price"` // DECIMAL(10,2) as string
	ImageURL               *string                  `db:"image_url" json:"image_url,omitempty"`
	StockStatus            LoungeProductStockStatus `db:"stock_status" json:"stock_status"`
	ProductType            LoungeProductType        `db:"product_type" json:"product_type"`
	IsAvailable            bool                     `db:"is_available" json:"is_available"`
	IsPreOrderable         bool                     `db:"is_pre_orderable" json:"is_pre_orderable"`
	IsVegetarian           bool                     `db:"is_vegetarian" json:"is_vegetarian"`
	IsHalal                bool                     `db:"is_halal" json:"is_halal"`
	DisplayOrder           int                      `db:"display_order" json:"display_order"`
	ServiceDurationMinutes *int                     `db:"service_duration_minutes" json:"service_duration_minutes,omitempty"`
	AvailableFrom          *string                  `db:"available_from" json:"available_from,omitempty"`   // TIME
	AvailableUntil         *string                  `db:"available_until" json:"available_until,omitempty"` // TIME
	Tags                   []string                 `db:"tags" json:"tags,omitempty"`                       // TEXT[]
	CreatedAt              time.Time                `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time                `db:"updated_at" json:"updated_at"`

	// Populated via JOINs
	CategoryName string `db:"-" json:"category_name,omitempty"`
}

// ============================================================================
// LOUNGE PROMOTION (lounge_promotions table)
// ============================================================================

// LoungePromotion represents a promotional code for lounge services
type LoungePromotion struct {
	ID                uuid.UUID      `db:"id" json:"id"`
	LoungeID          *uuid.UUID     `db:"lounge_id" json:"lounge_id,omitempty"` // NULL = applies to all lounges
	Code              string         `db:"code" json:"code"`
	Description       sql.NullString `db:"description" json:"description,omitempty"`
	DiscountType      string         `db:"discount_type" json:"discount_type"`   // 'percentage' or 'fixed'
	DiscountValue     string         `db:"discount_value" json:"discount_value"` // DECIMAL(10,2)
	MinOrderAmount    sql.NullString `db:"min_order_amount" json:"min_order_amount,omitempty"`
	MaxDiscountAmount sql.NullString `db:"max_discount_amount" json:"max_discount_amount,omitempty"`
	ValidFrom         time.Time      `db:"valid_from" json:"valid_from"`
	ValidUntil        time.Time      `db:"valid_until" json:"valid_until"`
	MaxUsageCount     sql.NullInt64  `db:"max_usage_count" json:"max_usage_count,omitempty"`
	CurrentUsageCount int            `db:"current_usage_count" json:"current_usage_count"`
	IsActive          bool           `db:"is_active" json:"is_active"`
	CreatedAt         time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// LOUNGE BOOKING (lounge_bookings table)
// ============================================================================

// LoungeBooking represents a lounge reservation
type LoungeBooking struct {
	ID               uuid.UUID         `db:"id" json:"id"`
	BookingReference string            `db:"booking_reference" json:"booking_reference"`
	UserID           uuid.UUID         `db:"user_id" json:"user_id"`
	LoungeID         uuid.UUID         `db:"lounge_id" json:"lounge_id"`
	MasterBookingID  *uuid.UUID        `db:"master_booking_id" json:"master_booking_id,omitempty"`
	BusBookingID     *uuid.UUID        `db:"bus_booking_id" json:"bus_booking_id,omitempty"`
	BookingType      LoungeBookingType `db:"booking_type" json:"booking_type"`

	// Timing
	ScheduledArrival   time.Time    `db:"scheduled_arrival" json:"scheduled_arrival"`
	ScheduledDeparture sql.NullTime `db:"scheduled_departure" json:"scheduled_departure,omitempty"`
	ActualArrival      sql.NullTime `db:"actual_arrival" json:"actual_arrival,omitempty"`
	ActualDeparture    sql.NullTime `db:"actual_departure" json:"actual_departure,omitempty"`

	// Guests
	NumberOfGuests int `db:"number_of_guests" json:"number_of_guests"`

	// Pricing
	PricingType    string `db:"pricing_type" json:"pricing_type"` // '1_hour', '2_hours', '3_hours', 'until_bus'
	BasePrice      string `db:"base_price" json:"base_price"`     // DECIMAL
	PreOrderTotal  string `db:"pre_order_total" json:"pre_order_total"`
	DiscountAmount string `db:"discount_amount" json:"discount_amount"`
	TotalAmount    string `db:"total_amount" json:"total_amount"`

	// Status & Payment
	Status        LoungeBookingStatus `db:"status" json:"status"`
	PaymentStatus LoungePaymentStatus `db:"payment_status" json:"payment_status"`

	// Contact
	PrimaryGuestName  string `db:"primary_guest_name" json:"primary_guest_name"`
	PrimaryGuestPhone string `db:"primary_guest_phone" json:"primary_guest_phone"`

	// Promo
	PromoCode sql.NullString `db:"promo_code" json:"promo_code,omitempty"`

	// Notes
	SpecialRequests sql.NullString `db:"special_requests" json:"special_requests,omitempty"`
	InternalNotes   sql.NullString `db:"internal_notes" json:"internal_notes,omitempty"`

	// Lounge Info (denormalized for booking record)
	LoungeName    string         `db:"lounge_name" json:"lounge_name"`
	LoungeAddress sql.NullString `db:"lounge_address" json:"lounge_address,omitempty"`
	LoungePhone   sql.NullString `db:"lounge_phone" json:"lounge_phone,omitempty"`
	PricePerGuest string         `db:"price_per_guest" json:"price_per_guest"`

	// Timestamps
	CancelledAt        sql.NullTime   `db:"cancelled_at" json:"cancelled_at,omitempty"`
	CancellationReason sql.NullString `db:"cancellation_reason" json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at" json:"updated_at"`

	// Populated via JOINs (not in DB table itself)
	Guests    []LoungeBookingGuest    `db:"-" json:"guests,omitempty"`
	PreOrders []LoungeBookingPreOrder `db:"-" json:"pre_orders,omitempty"`
}

// ============================================================================
// LOUNGE BOOKING GUEST (lounge_booking_guests table)
// ============================================================================

// LoungeBookingGuest represents a guest in a lounge booking
type LoungeBookingGuest struct {
	ID               uuid.UUID      `db:"id" json:"id"`
	LoungeBookingID  uuid.UUID      `db:"lounge_booking_id" json:"lounge_booking_id"`
	GuestName        string         `db:"guest_name" json:"guest_name"`
	GuestPhone       sql.NullString `db:"guest_phone" json:"guest_phone,omitempty"`
	IsPrimaryGuest   bool           `db:"is_primary_guest" json:"is_primary_guest"`
	CheckedInAt      sql.NullTime   `db:"checked_in_at" json:"checked_in_at,omitempty"`
	CheckedInByStaff *uuid.UUID     `db:"checked_in_by_staff" json:"checked_in_by_staff,omitempty"`
	CreatedAt        time.Time      `db:"created_at" json:"created_at"`
}

// ============================================================================
// LOUNGE BOOKING PRE-ORDER (lounge_booking_pre_orders table)
// ============================================================================

// LoungeBookingPreOrder represents a pre-ordered item with a booking
type LoungeBookingPreOrder struct {
	ID              uuid.UUID `db:"id" json:"id"`
	LoungeBookingID uuid.UUID `db:"lounge_booking_id" json:"lounge_booking_id"`
	ProductID       uuid.UUID `db:"product_id" json:"product_id"`
	ProductName     string    `db:"product_name" json:"product_name"` // Snapshot at booking time
	Quantity        int       `db:"quantity" json:"quantity"`
	UnitPrice       string    `db:"unit_price" json:"unit_price"`   // DECIMAL - snapshot
	TotalPrice      string    `db:"total_price" json:"total_price"` // DECIMAL
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

// ============================================================================
// LOUNGE ORDER (lounge_orders table) - In-lounge orders after check-in
// ============================================================================

// LoungeOrder represents an order placed while inside the lounge
type LoungeOrder struct {
	ID              uuid.UUID                `db:"id" json:"id"`
	LoungeBookingID uuid.UUID                `db:"lounge_booking_id" json:"lounge_booking_id"`
	LoungeID        uuid.UUID                `db:"lounge_id" json:"lounge_id"`
	OrderNumber     string                   `db:"order_number" json:"order_number"`
	Subtotal        string                   `db:"subtotal" json:"subtotal"` // DECIMAL
	DiscountAmount  string                   `db:"discount_amount" json:"discount_amount"`
	TotalAmount     string                   `db:"total_amount" json:"total_amount"`
	Status          LoungeOrderStatus        `db:"status" json:"status"`
	PaymentStatus   LoungeOrderPaymentStatus `db:"payment_status" json:"payment_status"`
	PaymentMethod   sql.NullString           `db:"payment_method" json:"payment_method,omitempty"`
	Notes           sql.NullString           `db:"notes" json:"notes,omitempty"`
	PreparedByStaff *uuid.UUID               `db:"prepared_by_staff" json:"prepared_by_staff,omitempty"`
	ServedByStaff   *uuid.UUID               `db:"served_by_staff" json:"served_by_staff,omitempty"`
	CreatedAt       time.Time                `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time                `db:"updated_at" json:"updated_at"`

	// Populated via JOINs
	Items []LoungeOrderItem `db:"-" json:"items,omitempty"`
}

// ============================================================================
// LOUNGE ORDER ITEM (lounge_order_items table)
// ============================================================================

// LoungeOrderItem represents an item in a lounge order
type LoungeOrderItem struct {
	ID          uuid.UUID `db:"id" json:"id"`
	OrderID     uuid.UUID `db:"order_id" json:"order_id"`
	ProductID   uuid.UUID `db:"product_id" json:"product_id"`
	ProductName string    `db:"product_name" json:"product_name"` // Snapshot
	Quantity    int       `db:"quantity" json:"quantity"`
	UnitPrice   string    `db:"unit_price" json:"unit_price"`   // DECIMAL - snapshot
	TotalPrice  string    `db:"total_price" json:"total_price"` // DECIMAL
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// ============================================================================
// LOUNGE PRODUCT REVIEW (lounge_product_reviews table)
// ============================================================================

// LoungeProductReview represents a product review
type LoungeProductReview struct {
	ID              uuid.UUID      `db:"id" json:"id"`
	ProductID       uuid.UUID      `db:"product_id" json:"product_id"`
	UserID          uuid.UUID      `db:"user_id" json:"user_id"`
	LoungeBookingID *uuid.UUID     `db:"lounge_booking_id" json:"lounge_booking_id,omitempty"`
	Rating          int            `db:"rating" json:"rating"` // 1-5
	ReviewText      sql.NullString `db:"review_text" json:"review_text,omitempty"`
	IsVerified      bool           `db:"is_verified" json:"is_verified"`
	CreatedAt       time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updated_at"`

	// Populated via JOINs
	UserName    string `db:"-" json:"user_name,omitempty"`
	ProductName string `db:"-" json:"product_name,omitempty"`
}

// ============================================================================
// REQUEST/RESPONSE STRUCTS
// ============================================================================

// CreateLoungeBookingRequest is the request to create a lounge booking
type CreateLoungeBookingRequest struct {
	LoungeID     string  `json:"lounge_id" binding:"required"`
	BusBookingID *string `json:"bus_booking_id,omitempty"`        // For pre_trip/post_trip
	BookingType  string  `json:"booking_type" binding:"required"` // pre_trip, post_trip, standalone

	// Timing
	ScheduledArrival   string  `json:"scheduled_arrival" binding:"required"` // ISO 8601
	ScheduledDeparture *string `json:"scheduled_departure,omitempty"`

	// Guests
	NumberOfGuests int            `json:"number_of_guests" binding:"required,min=1"`
	Guests         []GuestRequest `json:"guests" binding:"required,min=1"`

	// Pricing
	PricingType string `json:"pricing_type" binding:"required"` // 1_hour, 2_hours, 3_hours, until_bus

	// Pre-orders
	PreOrders []PreOrderRequest `json:"pre_orders,omitempty"`

	// Contact
	PrimaryGuestName  string `json:"primary_guest_name" binding:"required"`
	PrimaryGuestPhone string `json:"primary_guest_phone" binding:"required"`

	// Promo
	PromoCode *string `json:"promo_code,omitempty"`

	// Special Requests
	SpecialRequests *string `json:"special_requests,omitempty"`
}

// GuestRequest represents a guest to add to a booking
type GuestRequest struct {
	GuestName      string  `json:"guest_name" binding:"required"`
	GuestPhone     *string `json:"guest_phone,omitempty"`
	IsPrimaryGuest bool    `json:"is_primary_guest"`
}

// PreOrderRequest represents a pre-order item
type PreOrderRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

// Validate validates the lounge booking request
func (r *CreateLoungeBookingRequest) Validate() error {
	if r.NumberOfGuests < 1 {
		return errors.New("at least one guest is required")
	}
	if len(r.Guests) == 0 {
		return errors.New("guest details are required")
	}
	if r.NumberOfGuests != len(r.Guests) {
		return errors.New("number_of_guests must match the number of guest details provided")
	}

	validBookingTypes := map[string]bool{
		"pre_trip": true, "post_trip": true, "standalone": true,
	}
	if !validBookingTypes[r.BookingType] {
		return errors.New("invalid booking_type: must be pre_trip, post_trip, or standalone")
	}

	validPricingTypes := map[string]bool{
		"1_hour": true, "2_hours": true, "3_hours": true, "until_bus": true,
	}
	if !validPricingTypes[r.PricingType] {
		return errors.New("invalid pricing_type")
	}

	// Ensure at least one primary guest
	hasPrimary := false
	for _, g := range r.Guests {
		if g.IsPrimaryGuest {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary && len(r.Guests) > 0 {
		r.Guests[0].IsPrimaryGuest = true
	}

	return nil
}

// CreateLoungeOrderRequest is the request to create an in-lounge order
type CreateLoungeOrderRequest struct {
	LoungeBookingID string             `json:"lounge_booking_id" binding:"required"`
	Items           []OrderItemRequest `json:"items" binding:"required,min=1"`
	Notes           *string            `json:"notes,omitempty"`
}

// OrderItemRequest represents an item to order
type OrderItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

// LoungeBookingResponse is the response after creating a booking
type LoungeBookingResponse struct {
	Booking   *LoungeBooking          `json:"booking"`
	Guests    []LoungeBookingGuest    `json:"guests,omitempty"`
	PreOrders []LoungeBookingPreOrder `json:"pre_orders,omitempty"`
}

// LoungeBookingListItem is a summary for listing bookings
type LoungeBookingListItem struct {
	ID               uuid.UUID           `json:"id" db:"id"`
	BookingReference string              `json:"booking_reference" db:"booking_reference"`
	LoungeID         uuid.UUID           `json:"lounge_id" db:"lounge_id"`
	LoungeName       string              `json:"lounge_name" db:"lounge_name"`
	BookingType      LoungeBookingType   `json:"booking_type" db:"booking_type"`
	ScheduledArrival time.Time           `json:"scheduled_arrival" db:"scheduled_arrival"`
	NumberOfGuests   int                 `json:"number_of_guests" db:"number_of_guests"`
	TotalAmount      string              `json:"total_amount" db:"total_amount"`
	Status           LoungeBookingStatus `json:"status" db:"status"`
	PaymentStatus    LoungePaymentStatus `json:"payment_status" db:"payment_status"`
	CreatedAt        time.Time           `json:"created_at" db:"created_at"`
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// CanBeCancelled checks if lounge booking can be cancelled
func (b *LoungeBooking) CanBeCancelled() bool {
	return b.Status == LoungeBookingStatusPending ||
		b.Status == LoungeBookingStatusConfirmed
}

// CanCheckIn checks if guests can check in
func (b *LoungeBooking) CanCheckIn() bool {
	return b.Status == LoungeBookingStatusConfirmed
}

// IsActive checks if booking is currently active
func (b *LoungeBooking) IsActive() bool {
	return b.Status == LoungeBookingStatusConfirmed ||
		b.Status == LoungeBookingStatusCheckedIn
}

// GenerateBookingReference generates a unique booking reference
func GenerateLoungeBookingReference() string {
	// Format: LNG-XXXXXX (6 alphanumeric characters)
	id := uuid.New()
	return "LNG-" + id.String()[0:6]
}

// GenerateOrderNumber generates a unique order number
func GenerateLoungeOrderNumber() string {
	// Format: ORD-XXXXXX
	id := uuid.New()
	return "ORD-" + id.String()[0:6]
}
