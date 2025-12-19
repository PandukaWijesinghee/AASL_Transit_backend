package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeBookingHandler handles lounge booking-related HTTP requests
type LoungeBookingHandler struct {
	bookingRepo     *database.LoungeBookingRepository
	loungeRepo      *database.LoungeRepository
	loungeOwnerRepo *database.LoungeOwnerRepository
}

// NewLoungeBookingHandler creates a new lounge booking handler
func NewLoungeBookingHandler(
	bookingRepo *database.LoungeBookingRepository,
	loungeRepo *database.LoungeRepository,
	loungeOwnerRepo *database.LoungeOwnerRepository,
) *LoungeBookingHandler {
	return &LoungeBookingHandler{
		bookingRepo:     bookingRepo,
		loungeRepo:      loungeRepo,
		loungeOwnerRepo: loungeOwnerRepo,
	}
}

// ============================================================================
// MARKETPLACE CATEGORIES
// ============================================================================

// GetCategories handles GET /api/v1/lounge-marketplace/categories
func (h *LoungeBookingHandler) GetCategories(c *gin.Context) {
	categories, err := h.bookingRepo.GetAllCategories()
	if err != nil {
		log.Printf("ERROR: Failed to get categories: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve categories",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
		"total":      len(categories),
	})
}

// ============================================================================
// LOUNGE PRODUCTS
// ============================================================================

// GetLoungeProducts handles GET /api/v1/lounges/:id/products
func (h *LoungeBookingHandler) GetLoungeProducts(c *gin.Context) {
	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	products, err := h.bookingRepo.GetProductsByLoungeID(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get products for lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve products",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"products":  products,
		"lounge_id": loungeID,
		"total":     len(products),
	})
}

// CreateProductRequest represents the request to create a product
type CreateProductRequest struct {
	CategoryID             string   `json:"category_id" binding:"required"`
	Name                   string   `json:"name" binding:"required"`
	Description            *string  `json:"description,omitempty"`
	ProductType            string   `json:"product_type"`
	Price                  string   `json:"price" binding:"required"`
	DiscountedPrice        *string  `json:"discounted_price,omitempty"`
	ImageURL               *string  `json:"image_url,omitempty"`
	ThumbnailURL           *string  `json:"thumbnail_url,omitempty"`
	StockStatus            string   `json:"stock_status"`
	StockQuantity          *int     `json:"stock_quantity,omitempty"`
	IsAvailable            *bool    `json:"is_available,omitempty"`
	IsPreOrderable         *bool    `json:"is_pre_orderable,omitempty"`
	AvailableFrom          *string  `json:"available_from,omitempty"`
	AvailableUntil         *string  `json:"available_until,omitempty"`
	AvailableDays          []string `json:"available_days,omitempty"`
	ServiceDurationMinutes *int     `json:"service_duration_minutes,omitempty"`
	IsVegetarian           *bool    `json:"is_vegetarian,omitempty"`
	IsVegan                *bool    `json:"is_vegan,omitempty"`
	IsHalal                *bool    `json:"is_halal,omitempty"`
	Allergens              []string `json:"allergens,omitempty"`
	Calories               *int     `json:"calories,omitempty"`
	DisplayOrder           int      `json:"display_order"`
	IsFeatured             *bool    `json:"is_featured,omitempty"`
	Tags                   []string `json:"tags,omitempty"`
}

// CreateProduct handles POST /api/v1/lounges/:id/products (lounge owner only)
func (h *LoungeBookingHandler) CreateProduct(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	// Verify ownership
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not a lounge owner",
		})
		return
	}

	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't own this lounge",
		})
		return
	}

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid category_id format",
		})
		return
	}

	product := &models.LoungeProduct{
		LoungeID:     loungeID,
		CategoryID:   categoryID,
		Name:         req.Name,
		Price:        req.Price,
		DisplayOrder: req.DisplayOrder,
	}

	// Set optional fields
	if req.Description != nil {
		product.Description = req.Description
	}
	if req.ProductType != "" {
		product.ProductType = models.LoungeProductType(req.ProductType)
	} else {
		product.ProductType = models.LoungeProductTypeProduct
	}
	if req.DiscountedPrice != nil {
		product.DiscountedPrice = req.DiscountedPrice
	}
	if req.ImageURL != nil {
		product.ImageURL = req.ImageURL
	}
	if req.ThumbnailURL != nil {
		product.ThumbnailURL = req.ThumbnailURL
	}
	if req.StockStatus != "" {
		product.StockStatus = models.LoungeProductStockStatus(req.StockStatus)
	} else {
		product.StockStatus = models.LoungeProductStockStatusInStock
	}
	if req.StockQuantity != nil {
		product.StockQuantity = req.StockQuantity
	}
	if req.IsAvailable != nil {
		product.IsAvailable = *req.IsAvailable
	} else {
		product.IsAvailable = true
	}
	if req.IsPreOrderable != nil {
		product.IsPreOrderable = *req.IsPreOrderable
	}
	if req.AvailableFrom != nil {
		product.AvailableFrom = req.AvailableFrom
	}
	if req.AvailableUntil != nil {
		product.AvailableUntil = req.AvailableUntil
	}
	if len(req.AvailableDays) > 0 {
		product.AvailableDays = req.AvailableDays
	}
	if req.ServiceDurationMinutes != nil {
		product.ServiceDurationMinutes = req.ServiceDurationMinutes
	}
	if req.IsVegetarian != nil {
		product.IsVegetarian = *req.IsVegetarian
	}
	if req.IsVegan != nil {
		product.IsVegan = *req.IsVegan
	}
	if req.IsHalal != nil {
		product.IsHalal = *req.IsHalal
	}
	if len(req.Allergens) > 0 {
		product.Allergens = req.Allergens
	}
	if req.Calories != nil {
		product.Calories = req.Calories
	}
	if req.IsFeatured != nil {
		product.IsFeatured = *req.IsFeatured
	}
	if len(req.Tags) > 0 {
		product.Tags = req.Tags
	}
	product.IsActive = true

	if err := h.bookingRepo.CreateProduct(product); err != nil {
		log.Printf("ERROR: Failed to create product: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "creation_failed",
			Message: "Failed to create product",
		})
		return
	}

	// Return full product object
	c.JSON(http.StatusCreated, gin.H{
		"message": "Product created successfully",
		"product": gin.H{
			"id":                       product.ID.String(),
			"lounge_id":                product.LoungeID.String(),
			"category_id":              product.CategoryID.String(),
			"name":                     product.Name,
			"description":              product.Description,
			"product_type":             string(product.ProductType),
			"price":                    product.Price,
			"discounted_price":         product.DiscountedPrice,
			"image_url":                product.ImageURL,
			"thumbnail_url":            product.ThumbnailURL,
			"stock_status":             string(product.StockStatus),
			"stock_quantity":           product.StockQuantity,
			"is_available":             product.IsAvailable,
			"is_pre_orderable":         product.IsPreOrderable,
			"available_from":           product.AvailableFrom,
			"available_until":          product.AvailableUntil,
			"available_days":           product.AvailableDays,
			"service_duration_minutes": product.ServiceDurationMinutes,
			"is_vegetarian":            product.IsVegetarian,
			"is_vegan":                 product.IsVegan,
			"is_halal":                 product.IsHalal,
			"allergens":                product.Allergens,
			"calories":                 product.Calories,
			"display_order":            product.DisplayOrder,
			"is_featured":              product.IsFeatured,
			"tags":                     product.Tags,
			"average_rating":           product.AverageRating,
			"total_reviews":            product.TotalReviews,
			"is_active":                product.IsActive,
			"created_at":               product.CreatedAt,
			"updated_at":               product.UpdatedAt,
		},
	})
}

// UpdateProductRequest represents the request to update a product
type UpdateProductRequest struct {
	CategoryID             string   `json:"category_id"`
	Name                   string   `json:"name"`
	Description            *string  `json:"description,omitempty"`
	ProductType            string   `json:"product_type"`
	Price                  string   `json:"price"`
	DiscountedPrice        *string  `json:"discounted_price,omitempty"`
	ImageURL               *string  `json:"image_url,omitempty"`
	ThumbnailURL           *string  `json:"thumbnail_url,omitempty"`
	StockStatus            string   `json:"stock_status"`
	StockQuantity          *int     `json:"stock_quantity,omitempty"`
	IsAvailable            *bool    `json:"is_available,omitempty"`
	IsPreOrderable         *bool    `json:"is_pre_orderable,omitempty"`
	AvailableFrom          *string  `json:"available_from,omitempty"`
	AvailableUntil         *string  `json:"available_until,omitempty"`
	AvailableDays          []string `json:"available_days,omitempty"`
	ServiceDurationMinutes *int     `json:"service_duration_minutes,omitempty"`
	IsVegetarian           *bool    `json:"is_vegetarian,omitempty"`
	IsVegan                *bool    `json:"is_vegan,omitempty"`
	IsHalal                *bool    `json:"is_halal,omitempty"`
	Allergens              []string `json:"allergens,omitempty"`
	Calories               *int     `json:"calories,omitempty"`
	DisplayOrder           int      `json:"display_order"`
	IsFeatured             *bool    `json:"is_featured,omitempty"`
	Tags                   []string `json:"tags,omitempty"`
}

// UpdateProduct handles PUT /api/v1/lounges/:id/products/:product_id
func (h *LoungeBookingHandler) UpdateProduct(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	productIDStr := c.Param("product_id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid product ID format",
		})
		return
	}

	// Verify ownership
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not a lounge owner",
		})
		return
	}

	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't own this lounge",
		})
		return
	}

	// Get existing product
	product, err := h.bookingRepo.GetProductByID(productID)
	if err != nil || product == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Product not found",
		})
		return
	}

	if product.LoungeID != loungeID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Product doesn't belong to this lounge",
		})
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Update fields
	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Price != "" {
		product.Price = req.Price
	}
	if req.CategoryID != "" {
		categoryID, _ := uuid.Parse(req.CategoryID)
		product.CategoryID = categoryID
	}
	if req.Description != nil {
		product.Description = req.Description
	}
	if req.ProductType != "" {
		product.ProductType = models.LoungeProductType(req.ProductType)
	}
	if req.DiscountedPrice != nil {
		product.DiscountedPrice = req.DiscountedPrice
	}
	if req.ImageURL != nil {
		product.ImageURL = req.ImageURL
	}
	if req.ThumbnailURL != nil {
		product.ThumbnailURL = req.ThumbnailURL
	}
	if req.StockStatus != "" {
		product.StockStatus = models.LoungeProductStockStatus(req.StockStatus)
	}
	if req.StockQuantity != nil {
		product.StockQuantity = req.StockQuantity
	}
	if req.IsAvailable != nil {
		product.IsAvailable = *req.IsAvailable
	}
	if req.IsPreOrderable != nil {
		product.IsPreOrderable = *req.IsPreOrderable
	}
	if req.AvailableFrom != nil {
		product.AvailableFrom = req.AvailableFrom
	}
	if req.AvailableUntil != nil {
		product.AvailableUntil = req.AvailableUntil
	}
	if len(req.AvailableDays) > 0 {
		product.AvailableDays = req.AvailableDays
	}
	if req.ServiceDurationMinutes != nil {
		product.ServiceDurationMinutes = req.ServiceDurationMinutes
	}
	if req.IsVegetarian != nil {
		product.IsVegetarian = *req.IsVegetarian
	}
	if req.IsVegan != nil {
		product.IsVegan = *req.IsVegan
	}
	if req.IsHalal != nil {
		product.IsHalal = *req.IsHalal
	}
	if len(req.Allergens) > 0 {
		product.Allergens = req.Allergens
	}
	if req.Calories != nil {
		product.Calories = req.Calories
	}
	product.DisplayOrder = req.DisplayOrder
	if req.IsFeatured != nil {
		product.IsFeatured = *req.IsFeatured
	}
	if len(req.Tags) > 0 {
		product.Tags = req.Tags
	}

	if err := h.bookingRepo.UpdateProduct(product); err != nil {
		log.Printf("ERROR: Failed to update product: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update product",
		})
		return
	}

	// Return full product object
	c.JSON(http.StatusOK, gin.H{
		"message": "Product updated successfully",
		"product": gin.H{
			"id":                       product.ID.String(),
			"lounge_id":                product.LoungeID.String(),
			"category_id":              product.CategoryID.String(),
			"category_name":            product.CategoryName,
			"name":                     product.Name,
			"description":              product.Description,
			"product_type":             string(product.ProductType),
			"price":                    product.Price,
			"discounted_price":         product.DiscountedPrice,
			"image_url":                product.ImageURL,
			"thumbnail_url":            product.ThumbnailURL,
			"stock_status":             string(product.StockStatus),
			"stock_quantity":           product.StockQuantity,
			"is_available":             product.IsAvailable,
			"is_pre_orderable":         product.IsPreOrderable,
			"available_from":           product.AvailableFrom,
			"available_until":          product.AvailableUntil,
			"available_days":           product.AvailableDays,
			"service_duration_minutes": product.ServiceDurationMinutes,
			"is_vegetarian":            product.IsVegetarian,
			"is_vegan":                 product.IsVegan,
			"is_halal":                 product.IsHalal,
			"allergens":                product.Allergens,
			"calories":                 product.Calories,
			"display_order":            product.DisplayOrder,
			"is_featured":              product.IsFeatured,
			"tags":                     product.Tags,
			"average_rating":           product.AverageRating,
			"total_reviews":            product.TotalReviews,
			"is_active":                product.IsActive,
			"created_at":               product.CreatedAt,
			"updated_at":               product.UpdatedAt,
		},
	})
}

// DeleteProduct handles DELETE /api/v1/lounges/:id/products/:product_id
func (h *LoungeBookingHandler) DeleteProduct(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	productIDStr := c.Param("product_id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid product ID format",
		})
		return
	}

	// Verify ownership
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not a lounge owner",
		})
		return
	}

	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't own this lounge",
		})
		return
	}

	// Verify product belongs to lounge
	product, err := h.bookingRepo.GetProductByID(productID)
	if err != nil || product == nil || product.LoungeID != loungeID {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Product not found",
		})
		return
	}

	if err := h.bookingRepo.DeleteProduct(productID); err != nil {
		log.Printf("ERROR: Failed to delete product: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete product",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
}

// ============================================================================
// LOUNGE BOOKINGS - PASSENGER ENDPOINTS
// ============================================================================

// CreateLoungeBooking handles POST /api/v1/lounge-bookings
func (h *LoungeBookingHandler) CreateLoungeBooking(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req models.CreateLoungeBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	// Parse lounge ID
	loungeID, err := uuid.Parse(req.LoungeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid lounge_id format",
		})
		return
	}

	// Verify lounge exists and is active
	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.Status != "approved" || !lounge.IsOperational {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "lounge_unavailable",
			Message: "This lounge is currently not accepting bookings",
		})
		return
	}

	// Parse scheduled arrival
	scheduledArrival, err := time.Parse(time.RFC3339, req.ScheduledArrival)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid scheduled_arrival format. Use ISO 8601 (RFC3339)",
		})
		return
	}

	// Get base price for the pricing type
	basePrice, err := h.bookingRepo.GetLoungePrice(loungeID, req.PricingType)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge price: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "pricing_error",
			Message: "Failed to retrieve lounge pricing",
		})
		return
	}

	// Calculate price per guest
	var basePriceVal float64
	basePriceVal, _ = strconv.ParseFloat(basePrice, 64)
	pricePerGuest := strconv.FormatFloat(basePriceVal/float64(req.NumberOfGuests), 'f', 2, 64)

	// Build booking with denormalized lounge info
	booking := &models.LoungeBooking{
		UserID:            userCtx.UserID,
		LoungeID:          loungeID,
		BookingType:       models.LoungeBookingType(req.BookingType),
		ScheduledArrival:  scheduledArrival,
		NumberOfGuests:    req.NumberOfGuests,
		PricingType:       req.PricingType,
		BasePrice:         basePrice,
		PricePerGuest:     pricePerGuest,
		PreOrderTotal:     "0.00",
		DiscountAmount:    "0.00",
		PrimaryGuestName:  req.PrimaryGuestName,
		PrimaryGuestPhone: req.PrimaryGuestPhone,
		// Denormalized lounge info (snapshot at booking time)
		LoungeName:    lounge.LoungeName,
		LoungeAddress: lounge.Description, // Using description as address since address is already populated
		LoungePhone:   lounge.ContactPhone,
	}

	// Set LoungeAddress properly
	booking.LoungeAddress.String = lounge.Address
	booking.LoungeAddress.Valid = true

	// Handle scheduled departure
	if req.ScheduledDeparture != nil {
		scheduledDeparture, err := time.Parse(time.RFC3339, *req.ScheduledDeparture)
		if err == nil {
			booking.ScheduledDeparture.Time = scheduledDeparture
			booking.ScheduledDeparture.Valid = true
		}
	}

	// Handle bus booking ID for pre_trip/post_trip
	if req.BusBookingID != nil {
		busBookingID, err := uuid.Parse(*req.BusBookingID)
		if err == nil {
			booking.BusBookingID = &busBookingID
		}
	}

	// Handle special requests
	if req.SpecialRequests != nil {
		booking.SpecialRequests.String = *req.SpecialRequests
		booking.SpecialRequests.Valid = true
	}

	// Handle promo code
	if req.PromoCode != nil {
		booking.PromoCode.String = *req.PromoCode
		booking.PromoCode.Valid = true
		// TODO: Validate promo code and calculate discount
	}

	// Build guests
	guests := make([]models.LoungeBookingGuest, len(req.Guests))
	for i, g := range req.Guests {
		guests[i] = models.LoungeBookingGuest{
			GuestName:      g.GuestName,
			IsPrimaryGuest: g.IsPrimaryGuest,
		}
		if g.GuestPhone != nil {
			guests[i].GuestPhone.String = *g.GuestPhone
			guests[i].GuestPhone.Valid = true
		}
	}

	// Build pre-orders and calculate total
	var preOrders []models.LoungeBookingPreOrder
	preOrderTotal := 0.0

	for _, po := range req.PreOrders {
		productID, err := uuid.Parse(po.ProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Invalid product_id in pre-orders",
			})
			return
		}

		product, err := h.bookingRepo.GetProductByID(productID)
		if err != nil || product == nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Product not found in pre-orders",
			})
			return
		}

		if product.LoungeID != loungeID {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Product doesn't belong to this lounge",
			})
			return
		}

		// Calculate total price
		unitPrice := product.Price
		// Parse price and calculate total (simplified - proper decimal handling recommended)
		var priceFloat float64
		_, _ = strconv.ParseFloat(unitPrice, 64)
		totalFloat := priceFloat * float64(po.Quantity)
		preOrderTotal += totalFloat

		preOrders = append(preOrders, models.LoungeBookingPreOrder{
			ProductID:       productID,
			ProductName:     product.Name,                // Snapshot
			ProductType:     string(product.ProductType), // Snapshot - required NOT NULL
			ProductImageURL: product.ImageURL,            // Snapshot
			Quantity:        po.Quantity,
			UnitPrice:       unitPrice, // Snapshot
			TotalPrice:      strconv.FormatFloat(totalFloat, 'f', 2, 64),
		})
	}

	booking.PreOrderTotal = strconv.FormatFloat(preOrderTotal, 'f', 2, 64)

	// Calculate total amount (basePrice + preOrderTotal - discount)
	var basePriceFloat, discountFloat float64
	basePriceFloat, _ = strconv.ParseFloat(basePrice, 64)
	discountFloat, _ = strconv.ParseFloat(booking.DiscountAmount, 64)
	totalAmount := basePriceFloat + preOrderTotal - discountFloat
	booking.TotalAmount = strconv.FormatFloat(totalAmount, 'f', 2, 64)

	// Create booking
	createdBooking, err := h.bookingRepo.CreateLoungeBooking(booking, guests, preOrders)
	if err != nil {
		log.Printf("ERROR: Failed to create lounge booking: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "creation_failed",
			Message: "Failed to create booking: " + err.Error(),
		})
		return
	}

	// Auto-confirm for now (no payment integration yet)
	_ = h.bookingRepo.ConfirmLoungeBooking(createdBooking.ID)
	createdBooking.Status = models.LoungeBookingStatusConfirmed

	log.Printf("INFO: Lounge booking created - Ref: %s, User: %s, Lounge: %s",
		createdBooking.BookingReference, userCtx.UserID, loungeID)

	c.JSON(http.StatusCreated, gin.H{
		"message":           "Booking created successfully",
		"booking_reference": createdBooking.BookingReference,
		"booking_id":        createdBooking.ID,
		"status":            createdBooking.Status,
		"total_amount":      createdBooking.TotalAmount,
		"booking":           createdBooking,
	})
}

// GetMyLoungeBookings handles GET /api/v1/lounge-bookings
// Supports optional ?status=completed|cancelled query parameter
func (h *LoungeBookingHandler) GetMyLoungeBookings(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	statusFilter := strings.ToLower(strings.TrimSpace(c.Query("status"))) // Optional: "completed", "cancelled", etc.

	if statusFilter != "" {
		validStatuses := map[string]bool{
			"pending":   true,
			"confirmed": true,
			"checked_in": true,
			"completed": true,
			"cancelled": true,
			"no_show":   true,
		}

		if !validStatuses[statusFilter] {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_status",
				Message: "Invalid status filter. Allowed: pending, confirmed, checked_in, completed, cancelled, no_show",
			})
			return
		}
	}

	var bookings []models.LoungeBookingListItem
	var err error

	if statusFilter != "" {
		// Filter by specific status
		bookings, err = h.bookingRepo.GetLoungeBookingsByUserIDAndStatus(userCtx.UserID, statusFilter, limit, offset)
	} else {
		// Get all bookings
		bookings, err = h.bookingRepo.GetLoungeBookingsByUserID(userCtx.UserID, limit, offset)
	}

	if err != nil {
		log.Printf("ERROR: Failed to get lounge bookings: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve bookings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookings": bookings,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetUpcomingLoungeBookings handles GET /api/v1/lounge-bookings/upcoming
func (h *LoungeBookingHandler) GetUpcomingLoungeBookings(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	bookings, err := h.bookingRepo.GetUpcomingLoungeBookingsByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get upcoming lounge bookings: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve bookings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bookings": bookings})
}

// GetLoungeBookingByID handles GET /api/v1/lounge-bookings/:id
func (h *LoungeBookingHandler) GetLoungeBookingByID(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	bookingIDStr := c.Param("id")
	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid booking ID format",
		})
		return
	}

	booking, err := h.bookingRepo.GetLoungeBookingByID(bookingID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge booking: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve booking",
		})
		return
	}

	if booking == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Booking not found",
		})
		return
	}

	// Check ownership (user can view their own booking)
	if booking.UserID != userCtx.UserID {
		// Check if user is lounge owner or staff
		owner, _ := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
		lounge, _ := h.loungeRepo.GetLoungeByID(booking.LoungeID)
		if owner == nil || lounge == nil || lounge.LoungeOwnerID != owner.ID {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "forbidden",
				Message: "Not authorized to view this booking",
			})
			return
		}
	}

	c.JSON(http.StatusOK, booking)
}

// GetLoungeBookingByReference handles GET /api/v1/lounge-bookings/reference/:reference
func (h *LoungeBookingHandler) GetLoungeBookingByReference(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	reference := c.Param("reference")
	booking, err := h.bookingRepo.GetLoungeBookingByReference(reference)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge booking by reference: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve booking",
		})
		return
	}

	if booking == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Booking not found",
		})
		return
	}

	// Check ownership
	if booking.UserID != userCtx.UserID {
		owner, _ := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
		lounge, _ := h.loungeRepo.GetLoungeByID(booking.LoungeID)
		if owner == nil || lounge == nil || lounge.LoungeOwnerID != owner.ID {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "forbidden",
				Message: "Not authorized to view this booking",
			})
			return
		}
	}

	c.JSON(http.StatusOK, booking)
}

// CancelLoungeBookingRequest represents the cancellation request
type CancelLoungeBookingRequest struct {
	Reason string `json:"reason"`
}

// CancelLoungeBooking handles POST /api/v1/lounge-bookings/:id/cancel
func (h *LoungeBookingHandler) CancelLoungeBooking(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	bookingIDStr := c.Param("id")
	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid booking ID format",
		})
		return
	}

	var req CancelLoungeBookingRequest
	c.ShouldBindJSON(&req) // Reason is optional

	booking, err := h.bookingRepo.GetLoungeBookingByID(bookingID)
	if err != nil || booking == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Booking not found",
		})
		return
	}

	// Check ownership
	if booking.UserID != userCtx.UserID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not authorized to cancel this booking",
		})
		return
	}

	if !booking.CanBeCancelled() {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "cannot_cancel",
			Message: "This booking cannot be cancelled",
		})
		return
	}

	reason := &req.Reason
	if req.Reason == "" {
		reason = nil
	}

	if err := h.bookingRepo.CancelLoungeBooking(bookingID, reason); err != nil {
		log.Printf("ERROR: Failed to cancel lounge booking: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "cancel_failed",
			Message: "Failed to cancel booking",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Booking cancelled successfully",
		"booking_id": bookingID,
	})
}

// ============================================================================
// LOUNGE BOOKINGS - LOUNGE OWNER/STAFF ENDPOINTS
// ============================================================================

// GetLoungeBookingsForOwner handles GET /api/v1/lounges/:id/bookings
func (h *LoungeBookingHandler) GetLoungeBookingsForOwner(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	// Verify ownership
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not a lounge owner",
		})
		return
	}

	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't own this lounge",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	bookings, err := h.bookingRepo.GetLoungeBookingsByLoungeID(loungeID, limit, offset)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge bookings for owner: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve bookings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookings":  bookings,
		"lounge_id": loungeID,
		"limit":     limit,
		"offset":    offset,
	})
}

// GetTodaysBookings handles GET /api/v1/lounges/:id/bookings/today
func (h *LoungeBookingHandler) GetTodaysBookings(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	// Verify ownership/staff
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		// TODO: Check if user is lounge staff
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not authorized",
		})
		return
	}

	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil || lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not authorized",
		})
		return
	}

	bookings, err := h.bookingRepo.GetTodaysLoungeBookings(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get today's lounge bookings: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve bookings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookings":  bookings,
		"lounge_id": loungeID,
		"date":      time.Now().Format("2006-01-02"),
	})
}

// CheckInGuestRequest represents the check-in request
type CheckInGuestRequest struct {
	GuestID string `json:"guest_id" binding:"required"`
}

// CheckInGuest handles POST /api/v1/lounge-bookings/:id/check-in
func (h *LoungeBookingHandler) CheckInGuest(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	bookingIDStr := c.Param("id")
	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid booking ID format",
		})
		return
	}

	var req CheckInGuestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body",
		})
		return
	}

	guestID, err := uuid.Parse(req.GuestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid guest_id format",
		})
		return
	}

	// Get booking and verify ownership
	booking, err := h.bookingRepo.GetLoungeBookingByID(bookingID)
	if err != nil || booking == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Booking not found",
		})
		return
	}

	// Verify user is lounge owner/staff
	owner, _ := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	lounge, _ := h.loungeRepo.GetLoungeByID(booking.LoungeID)
	if owner == nil || lounge == nil || lounge.LoungeOwnerID != owner.ID {
		// TODO: Check if user is lounge staff
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not authorized",
		})
		return
	}

	if !booking.CanCheckIn() {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "cannot_checkin",
			Message: "Booking is not in confirmed status",
		})
		return
	}

	// Check in guest
	if err := h.bookingRepo.CheckInGuest(guestID, userCtx.UserID); err != nil {
		log.Printf("ERROR: Failed to check in guest: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "checkin_failed",
			Message: "Failed to check in guest",
		})
		return
	}

	// Update booking status if this is the first check-in
	if booking.Status == models.LoungeBookingStatusConfirmed {
		_ = h.bookingRepo.CheckInBooking(bookingID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Guest checked in successfully",
		"booking_id": bookingID,
		"guest_id":   guestID,
	})
}

// CompleteLoungeBooking handles POST /api/v1/lounge-bookings/:id/complete
func (h *LoungeBookingHandler) CompleteLoungeBooking(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	bookingIDStr := c.Param("id")
	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid booking ID format",
		})
		return
	}

	booking, err := h.bookingRepo.GetLoungeBookingByID(bookingID)
	if err != nil || booking == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Booking not found",
		})
		return
	}

	// Verify user is lounge owner/staff
	owner, _ := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	lounge, _ := h.loungeRepo.GetLoungeByID(booking.LoungeID)
	if owner == nil || lounge == nil || lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not authorized",
		})
		return
	}

	if booking.Status != models.LoungeBookingStatusCheckedIn {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "cannot_complete",
			Message: "Booking must be checked in before completing",
		})
		return
	}

	if err := h.bookingRepo.CompleteLoungeBooking(bookingID); err != nil {
		log.Printf("ERROR: Failed to complete lounge booking: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "complete_failed",
			Message: "Failed to complete booking",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Booking completed successfully",
		"booking_id": bookingID,
	})
}

// ============================================================================
// LOUNGE ORDERS (In-lounge orders)
// ============================================================================

// CreateLoungeOrder handles POST /api/v1/lounge-orders
func (h *LoungeBookingHandler) CreateLoungeOrder(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Log user context for auditing
	log.Printf("INFO: Creating lounge order for user %s", userCtx.UserID)

	var req models.CreateLoungeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	bookingID, err := uuid.Parse(req.LoungeBookingID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid lounge_booking_id format",
		})
		return
	}

	// Get booking
	booking, err := h.bookingRepo.GetLoungeBookingByID(bookingID)
	if err != nil || booking == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Booking not found",
		})
		return
	}

	// Verify booking is checked in
	if booking.Status != models.LoungeBookingStatusCheckedIn {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "booking_not_active",
			Message: "Orders can only be placed for checked-in bookings",
		})
		return
	}

	// Build order
	order := &models.LoungeOrder{
		LoungeBookingID: bookingID,
		LoungeID:        booking.LoungeID,
		DiscountAmount:  "0.00",
	}

	if req.Notes != nil {
		order.Notes.String = *req.Notes
		order.Notes.Valid = true
	}

	// Build items and calculate totals
	var items []models.LoungeOrderItem
	subtotal := 0.0

	for _, item := range req.Items {
		productID, err := uuid.Parse(item.ProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Invalid product_id in items",
			})
			return
		}

		product, err := h.bookingRepo.GetProductByID(productID)
		if err != nil || product == nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Product not found",
			})
			return
		}

		if product.LoungeID != booking.LoungeID {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Product doesn't belong to this lounge",
			})
			return
		}

		priceFloat, _ := strconv.ParseFloat(product.Price, 64)
		totalFloat := priceFloat * float64(item.Quantity)
		subtotal += totalFloat

		items = append(items, models.LoungeOrderItem{
			ProductID:   productID,
			ProductName: product.Name,
			Quantity:    item.Quantity,
			UnitPrice:   product.Price,
			TotalPrice:  strconv.FormatFloat(totalFloat, 'f', 2, 64),
		})
	}

	order.Subtotal = strconv.FormatFloat(subtotal, 'f', 2, 64)
	order.TotalAmount = strconv.FormatFloat(subtotal, 'f', 2, 64)

	// Create order
	createdOrder, err := h.bookingRepo.CreateLoungeOrder(order, items)
	if err != nil {
		log.Printf("ERROR: Failed to create lounge order: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "creation_failed",
			Message: "Failed to create order",
		})
		return
	}

	log.Printf("INFO: Lounge order created - Order#: %s, Booking: %s",
		createdOrder.OrderNumber, bookingID)

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Order created successfully",
		"order_number": createdOrder.OrderNumber,
		"order_id":     createdOrder.ID,
		"total_amount": createdOrder.TotalAmount,
		"order":        createdOrder,
	})
}

// GetBookingOrders handles GET /api/v1/lounge-bookings/:id/orders
func (h *LoungeBookingHandler) GetBookingOrders(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	bookingIDStr := c.Param("id")
	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid booking ID format",
		})
		return
	}

	booking, err := h.bookingRepo.GetLoungeBookingByID(bookingID)
	if err != nil || booking == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Booking not found",
		})
		return
	}

	// Check authorization
	if booking.UserID != userCtx.UserID {
		owner, _ := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
		lounge, _ := h.loungeRepo.GetLoungeByID(booking.LoungeID)
		if owner == nil || lounge == nil || lounge.LoungeOwnerID != owner.ID {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "forbidden",
				Message: "Not authorized",
			})
			return
		}
	}

	orders, err := h.bookingRepo.GetOrdersByBookingID(bookingID)
	if err != nil {
		log.Printf("ERROR: Failed to get booking orders: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve orders",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders":     orders,
		"booking_id": bookingID,
		"total":      len(orders),
	})
}

// UpdateOrderStatusRequest represents the request to update order status
type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateOrderStatus handles PUT /api/v1/lounge-orders/:id/status
func (h *LoungeBookingHandler) UpdateOrderStatus(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid order ID format",
		})
		return
	}

	var req UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body",
		})
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"pending": true, "confirmed": true, "preparing": true,
		"ready": true, "served": true, "completed": true, "cancelled": true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid status value",
		})
		return
	}

	// TODO: Verify user is lounge owner/staff for this order's lounge
	_ = userCtx

	if err := h.bookingRepo.UpdateOrderStatus(orderID, models.LoungeOrderStatus(req.Status)); err != nil {
		log.Printf("ERROR: Failed to update order status: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update order status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order status updated",
		"order_id": orderID,
		"status":   req.Status,
	})
}
