package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
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

// GenerateLoungeBookingQR generates a unique QR code for lounge booking
// Format: LQ-YYYYMMDDHHMMSS-XXXXXXXX (8 char alphanumeric)
// Example: LQ-20251206143022-A1B2C3D4
func (r *LoungeBookingRepository) GenerateLoungeBookingQR() (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		// Generate 8 random bytes and take first 8 hex chars
		randomBytes := make([]byte, 4)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		randomStr := strings.ToUpper(hex.EncodeToString(randomBytes))

		timestampStr := time.Now().Format("20060102150405")
		qrData := fmt.Sprintf("LQ-%s-%s", timestampStr, randomStr)

		// Check if exists
		var count int
		err := r.db.Get(&count, `SELECT COUNT(*) FROM lounge_bookings WHERE qr_code_data = $1`, qrData)
		if err != nil {
			return "", fmt.Errorf("failed to check QR uniqueness: %w", err)
		}

		if count == 0 {
			return qrData, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique lounge QR code after 10 attempts")
}

// ============================================================================
// MARKETPLACE CATEGORIES
// ============================================================================

// GetAllCategories returns all active marketplace categories
func (r *LoungeBookingRepository) GetAllCategories() ([]models.LoungeMarketplaceCategory, error) {
	query := `
		SELECT id, name, description, icon_name, icon_url, parent_category_id, 
		       display_order, is_active, created_at, updated_at
		FROM lounge_marketplace_categories
		WHERE is_active = TRUE
		ORDER BY display_order ASC
	`

	rows, err := r.db.Queryx(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.LoungeMarketplaceCategory
	for rows.Next() {
		var c models.LoungeMarketplaceCategory
		var description, iconName, iconURL sql.NullString
		var parentCategoryID uuid.NullUUID

		err := rows.Scan(
			&c.ID, &c.Name, &description, &iconName, &iconURL, &parentCategoryID,
			&c.DisplayOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if description.Valid {
			c.Description = &description.String
		}
		if iconName.Valid {
			c.IconName = &iconName.String
		}
		if iconURL.Valid {
			c.IconURL = &iconURL.String
		}
		if parentCategoryID.Valid {
			c.ParentCategoryID = &parentCategoryID.UUID
		}

		categories = append(categories, c)
	}

	return categories, nil
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
			p.product_type, p.price, p.discounted_price, p.image_url, p.thumbnail_url,
			p.stock_status, p.stock_quantity, p.is_available, p.is_pre_orderable,
			p.available_from, p.available_until, p.available_days,
			p.service_duration_minutes, p.is_vegetarian, p.is_vegan, p.is_halal,
			p.allergens, p.calories, p.display_order, p.is_featured, p.tags,
			p.average_rating, p.total_reviews, p.is_active,
			p.created_at, p.updated_at,
			c.name as category_name
		FROM lounge_products p
		JOIN lounge_marketplace_categories c ON p.category_id = c.id
		WHERE p.lounge_id = $1 AND p.is_active = TRUE AND p.is_available = TRUE
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
		var stockStatus, productType string
		var tags, availableDays, allergens []string

		// Use sql.Null* types for scanning, then convert to pointers
		var description, discountedPrice, imageURL, thumbnailURL sql.NullString
		var availableFrom, availableUntil, averageRating sql.NullString
		var serviceDurationMinutes, stockQuantity, calories sql.NullInt64

		err := rows.Scan(
			&p.ID, &p.LoungeID, &p.CategoryID, &p.Name, &description,
			&productType, &p.Price, &discountedPrice, &imageURL, &thumbnailURL,
			&stockStatus, &stockQuantity, &p.IsAvailable, &p.IsPreOrderable,
			&availableFrom, &availableUntil, pq.Array(&availableDays),
			&serviceDurationMinutes, &p.IsVegetarian, &p.IsVegan, &p.IsHalal,
			pq.Array(&allergens), &calories, &p.DisplayOrder, &p.IsFeatured, pq.Array(&tags),
			&averageRating, &p.TotalReviews, &p.IsActive,
			&p.CreatedAt, &p.UpdatedAt, &categoryName,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.Null* to pointers
		if description.Valid {
			p.Description = &description.String
		}
		if discountedPrice.Valid {
			p.DiscountedPrice = &discountedPrice.String
		}
		if imageURL.Valid {
			p.ImageURL = &imageURL.String
		}
		if thumbnailURL.Valid {
			p.ThumbnailURL = &thumbnailURL.String
		}
		if stockQuantity.Valid {
			val := int(stockQuantity.Int64)
			p.StockQuantity = &val
		}
		if serviceDurationMinutes.Valid {
			val := int(serviceDurationMinutes.Int64)
			p.ServiceDurationMinutes = &val
		}
		if availableFrom.Valid {
			p.AvailableFrom = &availableFrom.String
		}
		if availableUntil.Valid {
			p.AvailableUntil = &availableUntil.String
		}
		if calories.Valid {
			val := int(calories.Int64)
			p.Calories = &val
		}
		if averageRating.Valid {
			p.AverageRating = &averageRating.String
		}

		p.StockStatus = models.LoungeProductStockStatus(stockStatus)
		p.ProductType = models.LoungeProductType(productType)
		p.Tags = tags
		p.AvailableDays = availableDays
		p.Allergens = allergens
		p.CategoryName = categoryName
		products = append(products, p)
	}

	return products, nil
}

// GetProductByID returns a product by ID
func (r *LoungeBookingRepository) GetProductByID(productID uuid.UUID) (*models.LoungeProduct, error) {
	var p models.LoungeProduct
	query := `
		SELECT 
			p.id, p.lounge_id, p.category_id, p.name, p.description, 
			p.product_type, p.price, p.discounted_price, p.image_url, p.thumbnail_url,
			p.stock_status, p.stock_quantity, p.is_available, p.is_pre_orderable,
			p.available_from, p.available_until, p.available_days,
			p.service_duration_minutes, p.is_vegetarian, p.is_vegan, p.is_halal,
			p.allergens, p.calories, p.display_order, p.is_featured, p.tags,
			p.average_rating, p.total_reviews, p.is_active,
			p.created_at, p.updated_at,
			c.name as category_name
		FROM lounge_products p
		LEFT JOIN lounge_marketplace_categories c ON p.category_id = c.id
		WHERE p.id = $1
	`

	// Scan with proper type handling
	var description, discountedPrice, imageURL, thumbnailURL sql.NullString
	var availableFrom, availableUntil, averageRating sql.NullString
	var serviceDurationMinutes, stockQuantity, calories sql.NullInt64
	var tags, availableDays, allergens []string
	var stockStatus, productType, categoryName string

	err := r.db.QueryRow(query, productID).Scan(
		&p.ID, &p.LoungeID, &p.CategoryID, &p.Name, &description,
		&productType, &p.Price, &discountedPrice, &imageURL, &thumbnailURL,
		&stockStatus, &stockQuantity, &p.IsAvailable, &p.IsPreOrderable,
		&availableFrom, &availableUntil, pq.Array(&availableDays),
		&serviceDurationMinutes, &p.IsVegetarian, &p.IsVegan, &p.IsHalal,
		pq.Array(&allergens), &calories, &p.DisplayOrder, &p.IsFeatured, pq.Array(&tags),
		&averageRating, &p.TotalReviews, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt,
		&categoryName,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Convert nullable fields to pointers
	if description.Valid {
		p.Description = &description.String
	}
	if discountedPrice.Valid {
		p.DiscountedPrice = &discountedPrice.String
	}
	if imageURL.Valid {
		p.ImageURL = &imageURL.String
	}
	if thumbnailURL.Valid {
		p.ThumbnailURL = &thumbnailURL.String
	}
	if stockQuantity.Valid {
		val := int(stockQuantity.Int64)
		p.StockQuantity = &val
	}
	if serviceDurationMinutes.Valid {
		val := int(serviceDurationMinutes.Int64)
		p.ServiceDurationMinutes = &val
	}
	if availableFrom.Valid {
		p.AvailableFrom = &availableFrom.String
	}
	if availableUntil.Valid {
		p.AvailableUntil = &availableUntil.String
	}
	if calories.Valid {
		val := int(calories.Int64)
		p.Calories = &val
	}
	if averageRating.Valid {
		p.AverageRating = &averageRating.String
	}

	// Set ENUM types and other fields
	p.StockStatus = models.LoungeProductStockStatus(stockStatus)
	p.ProductType = models.LoungeProductType(productType)
	p.Tags = tags
	p.AvailableDays = availableDays
	p.Allergens = allergens
	p.CategoryName = categoryName

	return &p, nil
}

// CreateProduct creates a new product for a lounge
func (r *LoungeBookingRepository) CreateProduct(product *models.LoungeProduct) error {
	product.ID = uuid.New()
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()

	// Set defaults
	if product.StockStatus == "" {
		product.StockStatus = models.LoungeProductStockStatusInStock
	}
	if product.ProductType == "" {
		product.ProductType = models.LoungeProductTypeProduct
	}
	product.IsActive = true

	query := `
		INSERT INTO lounge_products (
			id, lounge_id, category_id, name, description, product_type,
			price, discounted_price, image_url, thumbnail_url,
			stock_status, stock_quantity, is_available, is_pre_orderable,
			available_from, available_until, available_days,
			service_duration_minutes, is_vegetarian, is_vegan, is_halal,
			allergens, calories, display_order, is_featured, tags,
			is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29
		)
	`
	_, err := r.db.Exec(query,
		product.ID, product.LoungeID, product.CategoryID, product.Name, product.Description, product.ProductType,
		product.Price, product.DiscountedPrice, product.ImageURL, product.ThumbnailURL,
		product.StockStatus, product.StockQuantity, product.IsAvailable, product.IsPreOrderable,
		product.AvailableFrom, product.AvailableUntil, pq.Array(product.AvailableDays),
		product.ServiceDurationMinutes, product.IsVegetarian, product.IsVegan, product.IsHalal,
		pq.Array(product.Allergens), product.Calories, product.DisplayOrder, product.IsFeatured, pq.Array(product.Tags),
		product.IsActive, product.CreatedAt, product.UpdatedAt,
	)
	return err
}

// UpdateProduct updates a product
func (r *LoungeBookingRepository) UpdateProduct(product *models.LoungeProduct) error {
	product.UpdatedAt = time.Now()
	query := `
		UPDATE lounge_products
		SET category_id = $2, name = $3, description = $4, product_type = $5,
		    price = $6, discounted_price = $7, image_url = $8, thumbnail_url = $9,
		    stock_status = $10, stock_quantity = $11, is_available = $12, is_pre_orderable = $13,
		    available_from = $14, available_until = $15, available_days = $16,
		    service_duration_minutes = $17, is_vegetarian = $18, is_vegan = $19, is_halal = $20,
		    allergens = $21, calories = $22, display_order = $23, is_featured = $24, tags = $25,
		    updated_at = $26
		WHERE id = $1
	`
	_, err := r.db.Exec(query,
		product.ID, product.CategoryID, product.Name, product.Description, product.ProductType,
		product.Price, product.DiscountedPrice, product.ImageURL, product.ThumbnailURL,
		product.StockStatus, product.StockQuantity, product.IsAvailable, product.IsPreOrderable,
		product.AvailableFrom, product.AvailableUntil, pq.Array(product.AvailableDays),
		product.ServiceDurationMinutes, product.IsVegetarian, product.IsVegan, product.IsHalal,
		pq.Array(product.Allergens), product.Calories, product.DisplayOrder, product.IsFeatured, pq.Array(product.Tags),
		product.UpdatedAt,
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

	// Generate QR code for lounge booking
	qrCode, err := r.GenerateLoungeBookingQR()
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}
	booking.QRCodeData = &qrCode
	now := time.Now()
	booking.QRGeneratedAt = &now

	// Insert booking
	bookingQuery := `
		INSERT INTO lounge_bookings (
			id, booking_reference, user_id, lounge_id, master_booking_id, bus_booking_id,
			booking_type, scheduled_arrival, scheduled_departure, 
			number_of_guests, pricing_type, price_per_guest, base_price, pre_order_total, 
			discount_amount, total_amount, status, payment_status,
			lounge_name, lounge_address, lounge_phone,
			primary_guest_name, primary_guest_phone, promo_code, special_requests,
			qr_code_data, qr_generated_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29
		)
	`
	_, err = tx.Exec(bookingQuery,
		booking.ID, booking.BookingReference, booking.UserID, booking.LoungeID,
		booking.MasterBookingID, booking.BusBookingID, booking.BookingType,
		booking.ScheduledArrival, booking.ScheduledDeparture,
		booking.NumberOfGuests, booking.PricingType, booking.PricePerGuest, booking.BasePrice,
		booking.PreOrderTotal, booking.DiscountAmount, booking.TotalAmount,
		booking.Status, booking.PaymentStatus,
		booking.LoungeName, booking.LoungeAddress, booking.LoungePhone,
		booking.PrimaryGuestName, booking.PrimaryGuestPhone, booking.PromoCode, booking.SpecialRequests,
		booking.QRCodeData, booking.QRGeneratedAt,
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
		INSERT INTO lounge_booking_pre_orders (id, lounge_booking_id, product_id, product_name, product_type, product_image_url, quantity, unit_price, total_price, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	for i := range preOrders {
		preOrders[i].ID = uuid.New()
		preOrders[i].LoungeBookingID = booking.ID
		preOrders[i].CreatedAt = time.Now()

		_, err = tx.Exec(preOrderQuery,
			preOrders[i].ID, preOrders[i].LoungeBookingID, preOrders[i].ProductID,
			preOrders[i].ProductName, preOrders[i].ProductType, preOrders[i].ProductImageURL,
			preOrders[i].Quantity, preOrders[i].UnitPrice,
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
			lb.qr_code_data,
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
		&booking.QRCodeData,
		&booking.LoungeName, &booking.LoungeAddress,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan lounge booking %s: %w", bookingID, err)
	}

	// Get guests
	var guests []models.LoungeBookingGuest
	guestQuery := `
		SELECT id, lounge_booking_id, guest_name, guest_phone, is_primary_guest, checked_in_at, created_at
		FROM lounge_booking_guests
		WHERE lounge_booking_id = $1
		ORDER BY is_primary_guest DESC, created_at ASC
	`
	err = r.db.Select(&guests, guestQuery, bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guests for lounge booking %s: %w", bookingID, err)
	}
	booking.Guests = guests

	// Get pre-orders
	var preOrders []models.LoungeBookingPreOrder
	preOrderQuery := `
		SELECT id, lounge_booking_id, product_id, product_name, product_type, product_image_url, quantity, unit_price, total_price, created_at
		FROM lounge_booking_pre_orders
		WHERE lounge_booking_id = $1
		ORDER BY created_at ASC
	`
	err = r.db.Select(&preOrders, preOrderQuery, bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-orders for lounge booking %s: %w", bookingID, err)
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

// GetLoungeBookingsByUserIDAndStatus returns bookings for a user filtered by status
func (r *LoungeBookingRepository) GetLoungeBookingsByUserIDAndStatus(userID uuid.UUID, status string, limit, offset int) ([]models.LoungeBookingListItem, error) {
	var bookings []models.LoungeBookingListItem
	query := `
		SELECT 
			lb.id, lb.booking_reference, lb.lounge_id, l.lounge_name,
			lb.booking_type, lb.scheduled_arrival, lb.number_of_guests,
			lb.total_amount, lb.status, lb.payment_status, lb.created_at
		FROM lounge_bookings lb
		JOIN lounges l ON lb.lounge_id = l.id
		WHERE lb.user_id = $1
		  AND lb.status = $2
		ORDER BY lb.created_at DESC
		LIMIT $3 OFFSET $4
	`
	err := r.db.Select(&bookings, query, userID, status, limit, offset)
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
