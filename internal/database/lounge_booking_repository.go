package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeBookingRepository handles lounge booking database operations
type LoungeBookingRepository struct {
	db *sqlx.DB
}

// NewLoungeBookingRepository creates a new lounge booking repository
func NewLoungeBookingRepository(db *sqlx.DB) *LoungeBookingRepository {
	return &LoungeBookingRepository{db: db}
}

// ============================================================================
// MARKETPLACE CATEGORIES
// ============================================================================

// GetAllCategories returns all active marketplace categories
func (r *LoungeBookingRepository) GetAllCategories() ([]models.LoungeMarketplaceCategory, error) {
	var categories []models.LoungeMarketplaceCategory
	query := `
		SELECT id, name, description, icon_url, sort_order, is_active, created_at, updated_at
		FROM lounge_marketplace_categories
		WHERE is_active = TRUE
		ORDER BY sort_order ASC
	`
	err := r.db.Select(&categories, query)
	return categories, err
}

// ============================================================================
// LOUNGE PRODUCTS
// ============================================================================

// GetProductsByLoungeID returns all available products for a lounge
func (r *LoungeBookingRepository) GetProductsByLoungeID(loungeID uuid.UUID) ([]models.LoungeProduct, error) {
	var products []models.LoungeProduct
	query := `
		SELECT 
			p.id, p.lounge_id, p.category_id, p.name, p.description, 
			p.price, p.image_url, p.stock_status, p.product_type,
			p.is_available, p.is_pre_orderable, p.is_vegetarian, p.is_halal,
			p.display_order, p.service_duration_minutes, 
			p.available_from, p.available_until, p.tags,
			p.created_at, p.updated_at,
			c.name as category_name
		FROM lounge_products p
		JOIN lounge_marketplace_categories c ON p.category_id = c.id
		WHERE p.lounge_id = $1 AND p.is_available = TRUE
		ORDER BY c.display_order, p.display_order ASC
	`

	rows, err := r.db.Queryx(query, loungeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p models.LoungeProduct
		var categoryName string
		err := rows.Scan(
			&p.ID, &p.LoungeID, &p.CategoryID, &p.Name, &p.Description,
			&p.Price, &p.ImageURL, &p.StockStatus, &p.ProductType,
			&p.IsAvailable, &p.IsPreOrderable, &p.IsVegetarian, &p.IsHalal,
			&p.DisplayOrder, &p.ServiceDurationMinutes,
			&p.AvailableFrom, &p.AvailableUntil, &p.Tags,
			&p.CreatedAt, &p.UpdatedAt, &categoryName,
		)
		if err != nil {
			return nil, err
		}
		p.CategoryName = categoryName
		products = append(products, p)
	}

	return products, nil
}

// GetProductByID returns a product by ID
func (r *LoungeBookingRepository) GetProductByID(productID uuid.UUID) (*models.LoungeProduct, error) {
	var product models.LoungeProduct
	query := `
		SELECT id, lounge_id, category_id, name, description, price, 
		       image_url, is_available, sort_order, created_at, updated_at
		FROM lounge_products
		WHERE id = $1
	`
	err := r.db.Get(&product, query, productID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &product, err
}

// CreateProduct creates a new product for a lounge
func (r *LoungeBookingRepository) CreateProduct(product *models.LoungeProduct) error {
	product.ID = uuid.New()
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()

	query := `
		INSERT INTO lounge_products (
			id, lounge_id, category_id, name, description, price, 
			image_url, is_available, display_order, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(query,
		product.ID, product.LoungeID, product.CategoryID, product.Name, product.Description,
		product.Price, product.ImageURL, product.IsAvailable, product.DisplayOrder,
		product.CreatedAt, product.UpdatedAt,
	)
	return err
}

// UpdateProduct updates a product
func (r *LoungeBookingRepository) UpdateProduct(product *models.LoungeProduct) error {
	product.UpdatedAt = time.Now()
	query := `
		UPDATE lounge_products
		SET name = $2, description = $3, price = $4, image_url = $5, 
		    is_available = $6, display_order = $7, category_id = $8, updated_at = $9
		WHERE id = $1
	`
	_, err := r.db.Exec(query,
		product.ID, product.Name, product.Description, product.Price, product.ImageURL,
		product.IsAvailable, product.DisplayOrder, product.CategoryID, product.UpdatedAt,
	)
	return err
}

// DeleteProduct soft-deletes a product (sets is_available = false)
func (r *LoungeBookingRepository) DeleteProduct(productID uuid.UUID) error {
	query := `UPDATE lounge_products SET is_available = FALSE, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, productID)
	return err
}

// ============================================================================
// LOUNGE BOOKINGS
// ============================================================================

// CreateLoungeBooking creates a new lounge booking with guests and pre-orders
func (r *LoungeBookingRepository) CreateLoungeBooking(
	booking *models.LoungeBooking,
	guests []models.LoungeBookingGuest,
	preOrders []models.LoungeBookingPreOrder,
) (*models.LoungeBooking, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Generate booking reference and ID
	booking.ID = uuid.New()
	booking.BookingReference = models.GenerateLoungeBookingReference()
	booking.Status = models.LoungeBookingStatusPending
	booking.PaymentStatus = models.LoungePaymentPending
	booking.CreatedAt = time.Now()
	booking.UpdatedAt = time.Now()

	// Insert booking
	bookingQuery := `
		INSERT INTO lounge_bookings (
			id, booking_reference, user_id, lounge_id, master_booking_id, bus_booking_id,
			booking_type, scheduled_arrival, scheduled_departure, 
			number_of_guests, pricing_type, base_price, pre_order_total, 
			discount_amount, total_amount, status, payment_status,
			primary_guest_name, primary_guest_phone, promo_code, special_requests,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
		)
	`
	_, err = tx.Exec(bookingQuery,
		booking.ID, booking.BookingReference, booking.UserID, booking.LoungeID,
		booking.MasterBookingID, booking.BusBookingID, booking.BookingType,
		booking.ScheduledArrival, booking.ScheduledDeparture,
		booking.NumberOfGuests, booking.PricingType, booking.BasePrice,
		booking.PreOrderTotal, booking.DiscountAmount, booking.TotalAmount,
		booking.Status, booking.PaymentStatus, booking.PrimaryGuestName,
		booking.PrimaryGuestPhone, booking.PromoCode, booking.SpecialRequests,
		booking.CreatedAt, booking.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert booking: %w", err)
	}

	// Insert guests
	guestQuery := `
		INSERT INTO lounge_booking_guests (id, lounge_booking_id, guest_name, guest_phone, is_primary_guest, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	for i := range guests {
		guests[i].ID = uuid.New()
		guests[i].LoungeBookingID = booking.ID
		guests[i].CreatedAt = time.Now()

		_, err = tx.Exec(guestQuery,
			guests[i].ID, guests[i].LoungeBookingID, guests[i].GuestName,
			guests[i].GuestPhone, guests[i].IsPrimaryGuest, guests[i].CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert guest: %w", err)
		}
	}

	// Insert pre-orders
	preOrderQuery := `
		INSERT INTO lounge_booking_pre_orders (id, lounge_booking_id, product_id, product_name, quantity, unit_price, total_price, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for i := range preOrders {
		preOrders[i].ID = uuid.New()
		preOrders[i].LoungeBookingID = booking.ID
		preOrders[i].CreatedAt = time.Now()

		_, err = tx.Exec(preOrderQuery,
			preOrders[i].ID, preOrders[i].LoungeBookingID, preOrders[i].ProductID,
			preOrders[i].ProductName, preOrders[i].Quantity, preOrders[i].UnitPrice,
			preOrders[i].TotalPrice, preOrders[i].CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert pre-order: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	booking.Guests = guests
	booking.PreOrders = preOrders
	return booking, nil
}

// GetLoungeBookingByID returns a booking by ID with guests and pre-orders
func (r *LoungeBookingRepository) GetLoungeBookingByID(bookingID uuid.UUID) (*models.LoungeBooking, error) {
	var booking models.LoungeBooking
	query := `
		SELECT 
			lb.id, lb.booking_reference, lb.user_id, lb.lounge_id, lb.master_booking_id, lb.bus_booking_id,
			lb.booking_type, lb.scheduled_arrival, lb.scheduled_departure, lb.actual_arrival, lb.actual_departure,
			lb.number_of_guests, lb.pricing_type, lb.base_price, lb.pre_order_total,
			lb.discount_amount, lb.total_amount, lb.status, lb.payment_status,
			lb.primary_guest_name, lb.primary_guest_phone, lb.promo_code, lb.special_requests,
			lb.internal_notes, lb.cancelled_at, lb.cancellation_reason, lb.created_at, lb.updated_at,
			l.lounge_name, l.address
		FROM lounge_bookings lb
		JOIN lounges l ON lb.lounge_id = l.id
		WHERE lb.id = $1
	`

	row := r.db.QueryRow(query, bookingID)
	err := row.Scan(
		&booking.ID, &booking.BookingReference, &booking.UserID, &booking.LoungeID,
		&booking.MasterBookingID, &booking.BusBookingID, &booking.BookingType,
		&booking.ScheduledArrival, &booking.ScheduledDeparture, &booking.ActualArrival, &booking.ActualDeparture,
		&booking.NumberOfGuests, &booking.PricingType, &booking.BasePrice, &booking.PreOrderTotal,
		&booking.DiscountAmount, &booking.TotalAmount, &booking.Status, &booking.PaymentStatus,
		&booking.PrimaryGuestName, &booking.PrimaryGuestPhone, &booking.PromoCode, &booking.SpecialRequests,
		&booking.InternalNotes, &booking.CancelledAt, &booking.CancellationReason, &booking.CreatedAt, &booking.UpdatedAt,
		&booking.LoungeName, &booking.LoungeAddress,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Get guests
	var guests []models.LoungeBookingGuest
	guestQuery := `
		SELECT id, lounge_booking_id, guest_name, guest_phone, is_primary_guest, checked_in_at, checked_in_by_staff, created_at
		FROM lounge_booking_guests
		WHERE lounge_booking_id = $1
		ORDER BY is_primary_guest DESC, created_at ASC
	`
	err = r.db.Select(&guests, guestQuery, bookingID)
	if err != nil {
		return nil, err
	}
	booking.Guests = guests

	// Get pre-orders
	var preOrders []models.LoungeBookingPreOrder
	preOrderQuery := `
		SELECT id, lounge_booking_id, product_id, product_name, quantity, unit_price, total_price, created_at
		FROM lounge_booking_pre_orders
		WHERE lounge_booking_id = $1
		ORDER BY created_at ASC
	`
	err = r.db.Select(&preOrders, preOrderQuery, bookingID)
	if err != nil {
		return nil, err
	}
	booking.PreOrders = preOrders

	return &booking, nil
}

// GetLoungeBookingByReference returns a booking by reference
func (r *LoungeBookingRepository) GetLoungeBookingByReference(reference string) (*models.LoungeBooking, error) {
	var bookingID uuid.UUID
	query := `SELECT id FROM lounge_bookings WHERE booking_reference = $1`
	err := r.db.Get(&bookingID, query, reference)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetLoungeBookingByID(bookingID)
}

// GetLoungeBookingsByUserID returns all bookings for a user
func (r *LoungeBookingRepository) GetLoungeBookingsByUserID(userID uuid.UUID, limit, offset int) ([]models.LoungeBookingListItem, error) {
	var bookings []models.LoungeBookingListItem
	query := `
		SELECT 
			lb.id, lb.booking_reference, lb.lounge_id, l.lounge_name,
			lb.booking_type, lb.scheduled_arrival, lb.number_of_guests,
			lb.total_amount, lb.status, lb.payment_status, lb.created_at
		FROM lounge_bookings lb
		JOIN lounges l ON lb.lounge_id = l.id
		WHERE lb.user_id = $1
		ORDER BY lb.created_at DESC
		LIMIT $2 OFFSET $3
	`
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// GetUpcomingLoungeBookingsByUserID returns upcoming bookings for a user
func (r *LoungeBookingRepository) GetUpcomingLoungeBookingsByUserID(userID uuid.UUID) ([]models.LoungeBookingListItem, error) {
	var bookings []models.LoungeBookingListItem
	query := `
		SELECT 
			lb.id, lb.booking_reference, lb.lounge_id, l.lounge_name,
			lb.booking_type, lb.scheduled_arrival, lb.number_of_guests,
			lb.total_amount, lb.status, lb.payment_status, lb.created_at
		FROM lounge_bookings lb
		JOIN lounges l ON lb.lounge_id = l.id
		WHERE lb.user_id = $1 
		  AND lb.status IN ('pending', 'confirmed', 'checked_in')
		  AND lb.scheduled_arrival >= NOW()
		ORDER BY lb.scheduled_arrival ASC
	`
	err := r.db.Select(&bookings, query, userID)
	return bookings, err
}

// GetLoungeBookingsByLoungeID returns all bookings for a lounge (owner view)
func (r *LoungeBookingRepository) GetLoungeBookingsByLoungeID(loungeID uuid.UUID, limit, offset int) ([]models.LoungeBookingListItem, error) {
	var bookings []models.LoungeBookingListItem
	query := `
		SELECT 
			lb.id, lb.booking_reference, lb.lounge_id, l.lounge_name,
			lb.booking_type, lb.scheduled_arrival, lb.number_of_guests,
			lb.total_amount, lb.status, lb.payment_status, lb.created_at
		FROM lounge_bookings lb
		JOIN lounges l ON lb.lounge_id = l.id
		WHERE lb.lounge_id = $1
		ORDER BY lb.scheduled_arrival DESC
		LIMIT $2 OFFSET $3
	`
	err := r.db.Select(&bookings, query, loungeID, limit, offset)
	return bookings, err
}

// GetTodaysLoungeBookings returns today's bookings for a lounge
func (r *LoungeBookingRepository) GetTodaysLoungeBookings(loungeID uuid.UUID) ([]models.LoungeBookingListItem, error) {
	var bookings []models.LoungeBookingListItem
	query := `
		SELECT 
			lb.id, lb.booking_reference, lb.lounge_id, l.lounge_name,
			lb.booking_type, lb.scheduled_arrival, lb.number_of_guests,
			lb.total_amount, lb.status, lb.payment_status, lb.created_at
		FROM lounge_bookings lb
		JOIN lounges l ON lb.lounge_id = l.id
		WHERE lb.lounge_id = $1 
		  AND DATE(lb.scheduled_arrival) = CURRENT_DATE
		ORDER BY lb.scheduled_arrival ASC
	`
	err := r.db.Select(&bookings, query, loungeID)
	return bookings, err
}

// UpdateLoungeBookingStatus updates the status of a booking
func (r *LoungeBookingRepository) UpdateLoungeBookingStatus(bookingID uuid.UUID, status models.LoungeBookingStatus) error {
	query := `UPDATE lounge_bookings SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, bookingID, status)
	return err
}

// ConfirmLoungeBooking confirms a pending booking
func (r *LoungeBookingRepository) ConfirmLoungeBooking(bookingID uuid.UUID) error {
	return r.UpdateLoungeBookingStatus(bookingID, models.LoungeBookingStatusConfirmed)
}

// CancelLoungeBooking cancels a booking with reason
func (r *LoungeBookingRepository) CancelLoungeBooking(bookingID uuid.UUID, reason *string) error {
	query := `
		UPDATE lounge_bookings 
		SET status = 'cancelled', cancelled_at = NOW(), cancellation_reason = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(query, bookingID, reason)
	return err
}

// CheckInGuest marks a guest as checked in
func (r *LoungeBookingRepository) CheckInGuest(guestID uuid.UUID, staffID uuid.UUID) error {
	query := `
		UPDATE lounge_booking_guests 
		SET checked_in_at = NOW(), checked_in_by_staff = $2
		WHERE id = $1
	`
	_, err := r.db.Exec(query, guestID, staffID)
	return err
}

// CheckInBooking marks booking as checked in (when first guest checks in)
func (r *LoungeBookingRepository) CheckInBooking(bookingID uuid.UUID) error {
	query := `
		UPDATE lounge_bookings 
		SET status = 'checked_in', actual_arrival = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'confirmed'
	`
	result, err := r.db.Exec(query, bookingID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("booking not in confirmed status or not found")
	}
	return nil
}

// CompleteLoungeBooking marks a booking as completed
func (r *LoungeBookingRepository) CompleteLoungeBooking(bookingID uuid.UUID) error {
	query := `
		UPDATE lounge_bookings 
		SET status = 'completed', actual_departure = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'checked_in'
	`
	_, err := r.db.Exec(query, bookingID)
	return err
}

// UpdatePaymentStatus updates payment status
func (r *LoungeBookingRepository) UpdatePaymentStatus(bookingID uuid.UUID, status models.LoungePaymentStatus) error {
	query := `UPDATE lounge_bookings SET payment_status = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, bookingID, status)
	return err
}

// ============================================================================
// LOUNGE ORDERS (In-lounge orders after check-in)
// ============================================================================

// CreateLoungeOrder creates a new in-lounge order
func (r *LoungeBookingRepository) CreateLoungeOrder(order *models.LoungeOrder, items []models.LoungeOrderItem) (*models.LoungeOrder, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order.ID = uuid.New()
	order.OrderNumber = models.GenerateLoungeOrderNumber()
	order.Status = models.LoungeOrderStatusPending
	order.PaymentStatus = models.LoungeOrderPaymentStatusPending
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	orderQuery := `
		INSERT INTO lounge_orders (
			id, lounge_booking_id, lounge_id, order_number, subtotal, 
			discount_amount, total_amount, status, payment_status, notes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = tx.Exec(orderQuery,
		order.ID, order.LoungeBookingID, order.LoungeID, order.OrderNumber,
		order.Subtotal, order.DiscountAmount, order.TotalAmount,
		order.Status, order.PaymentStatus, order.Notes,
		order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	itemQuery := `
		INSERT INTO lounge_order_items (id, order_id, product_id, product_name, quantity, unit_price, total_price, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for i := range items {
		items[i].ID = uuid.New()
		items[i].OrderID = order.ID
		items[i].CreatedAt = time.Now()

		_, err = tx.Exec(itemQuery,
			items[i].ID, items[i].OrderID, items[i].ProductID,
			items[i].ProductName, items[i].Quantity, items[i].UnitPrice,
			items[i].TotalPrice, items[i].CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	order.Items = items
	return order, nil
}

// GetOrdersByBookingID returns all orders for a booking
func (r *LoungeBookingRepository) GetOrdersByBookingID(bookingID uuid.UUID) ([]models.LoungeOrder, error) {
	var orders []models.LoungeOrder
	query := `
		SELECT id, lounge_booking_id, lounge_id, order_number, subtotal, 
		       discount_amount, total_amount, status, payment_status, 
		       payment_method, notes, prepared_by_staff, served_by_staff, 
		       created_at, updated_at
		FROM lounge_orders
		WHERE lounge_booking_id = $1
		ORDER BY created_at DESC
	`
	err := r.db.Select(&orders, query, bookingID)
	if err != nil {
		return nil, err
	}

	// Get items for each order
	for i := range orders {
		var items []models.LoungeOrderItem
		itemQuery := `
			SELECT id, order_id, product_id, product_name, quantity, unit_price, total_price, created_at
			FROM lounge_order_items
			WHERE order_id = $1
			ORDER BY created_at ASC
		`
		err = r.db.Select(&items, itemQuery, orders[i].ID)
		if err != nil {
			return nil, err
		}
		orders[i].Items = items
	}

	return orders, nil
}

// UpdateOrderStatus updates order status
func (r *LoungeBookingRepository) UpdateOrderStatus(orderID uuid.UUID, status models.LoungeOrderStatus) error {
	query := `UPDATE lounge_orders SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, orderID, status)
	return err
}

// ============================================================================
// PROMOTIONS
// ============================================================================

// ValidatePromoCode validates a promo code for a lounge
func (r *LoungeBookingRepository) ValidatePromoCode(code string, loungeID *uuid.UUID) (*models.LoungePromotion, error) {
	var promo models.LoungePromotion
	query := `
		SELECT id, lounge_id, code, description, discount_type, discount_value, 
		       min_order_amount, max_discount_amount, valid_from, valid_until,
		       max_usage_count, current_usage_count, is_active, created_at, updated_at
		FROM lounge_promotions
		WHERE code = $1 
		  AND is_active = TRUE
		  AND valid_from <= NOW() 
		  AND valid_until >= NOW()
		  AND (lounge_id IS NULL OR lounge_id = $2)
		  AND (max_usage_count IS NULL OR current_usage_count < max_usage_count)
	`
	err := r.db.Get(&promo, query, code, loungeID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &promo, err
}

// IncrementPromoUsage increments the usage count for a promo
func (r *LoungeBookingRepository) IncrementPromoUsage(promoID uuid.UUID) error {
	query := `UPDATE lounge_promotions SET current_usage_count = current_usage_count + 1, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, promoID)
	return err
}

// ============================================================================
// LOUNGE INFO HELPER
// ============================================================================

// GetLoungePrice returns the price for a lounge based on pricing type
func (r *LoungeBookingRepository) GetLoungePrice(loungeID uuid.UUID, pricingType string) (string, error) {
	var price sql.NullString
	var query string

	switch pricingType {
	case "1_hour":
		query = `SELECT price_1_hour FROM lounges WHERE id = $1`
	case "2_hours":
		query = `SELECT price_2_hours FROM lounges WHERE id = $1`
	case "3_hours":
		query = `SELECT price_3_hours FROM lounges WHERE id = $1`
	case "until_bus":
		query = `SELECT price_until_bus FROM lounges WHERE id = $1`
	default:
		return "0.00", fmt.Errorf("invalid pricing type: %s", pricingType)
	}

	err := r.db.Get(&price, query, loungeID)
	if err != nil {
		return "0.00", err
	}

	if !price.Valid {
		return "0.00", nil
	}
	return price.String, nil
}
