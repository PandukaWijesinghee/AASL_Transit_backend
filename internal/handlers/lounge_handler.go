package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeHandler handles lounge-related HTTP requests
type LoungeHandler struct {
	loungeRepo      *database.LoungeRepository
	loungeOwnerRepo *database.LoungeOwnerRepository
}

// NewLoungeHandler creates a new lounge handler
func NewLoungeHandler(
	loungeRepo *database.LoungeRepository,
	loungeOwnerRepo *database.LoungeOwnerRepository,
) *LoungeHandler {
	return &LoungeHandler{
		loungeRepo:      loungeRepo,
		loungeOwnerRepo: loungeOwnerRepo,
	}
}

// ===================================================================
// ADD LOUNGE (STEP 3: Registration)
// ===================================================================

// AddLoungeRequest represents the lounge creation request
type AddLoungeRequest struct {
	LoungeName     string   `json:"lounge_name" binding:"required"`
	Address        string   `json:"address" binding:"required"`
	City           string   `json:"city" binding:"required"`
	ContactPhone   string   `json:"contact_phone" binding:"required"`
	Latitude       *string  `json:"latitude"`
	Longitude      *string  `json:"longitude"`
	Price1Hour     *string  `json:"price_1_hour"`      // DECIMAL as string (e.g., "500.00")
	Price2Hours    *string  `json:"price_2_hours"`     // DECIMAL as string (e.g., "900.00")
	PriceUntilBus  *string  `json:"price_until_bus"`   // DECIMAL as string (e.g., "1500.00")
	Amenities      []string `json:"amenities"`         // Array: ["WiFi", "AC", "Food"]
	Images         []string `json:"images"`            // Array of image URLs
}

// AddLounge handles POST /api/v1/lounge-owner/register/add-lounge
func (h *LoungeHandler) AddLounge(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req AddLoungeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge owner for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounge owner",
		})
		return
	}

	if owner == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge owner record not found",
		})
		return
	}

	// Check if previous steps are completed (must have uploaded NIC)
	if owner.RegistrationStep != models.RegStepNICUploaded && owner.RegistrationStep != models.RegStepLoungeAdded && owner.RegistrationStep != models.RegStepCompleted {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "incomplete_registration",
			Message: "Please complete previous registration steps first (business info and NIC upload required)",
		})
		return
	}

	// Convert amenities and images to JSONB
	amenitiesJSON, _ := json.Marshal(req.Amenities)
	imagesJSON, _ := json.Marshal(req.Images)

	// Create lounge
	lounge, err := h.loungeRepo.CreateLounge(
		owner.ID,
		req.LoungeName,
		req.Address,
		req.City,
		req.ContactPhone,
		req.Latitude,
		req.Longitude,
		req.Price1Hour,
		req.Price2Hours,
		req.PriceUntilBus,
		amenitiesJSON,
		imagesJSON,
	)
	if err != nil {
		log.Printf("ERROR: Failed to create lounge for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "creation_failed",
			Message: "Failed to create lounge: " + err.Error(),
		})
		return
	}

	// Update registration step to lounge_added (triggers will handle counts and profile completion)
	err = h.loungeOwnerRepo.UpdateRegistrationStep(userCtx.UserID, models.RegStepLoungeAdded)
	if err != nil {
		log.Printf("ERROR: Failed to update registration step: %v", err)
		// Continue anyway - trigger should have done it
	}

	log.Printf("INFO: Lounge created successfully for lounge owner %s (lounge_id: %s)", userCtx.UserID, lounge.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":           "Lounge added successfully",
		"lounge_id":         lounge.ID,
		"registration_step": models.RegStepLoungeAdded,
		"status":            lounge.Status,
	})
}

// ===================================================================
// GET MY LOUNGES
// ===================================================================

// GetMyLounges handles GET /api/v1/lounges/my-lounges
func (h *LoungeHandler) GetMyLounges(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge owner for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	if owner == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge owner not found",
		})
		return
	}

	// Get lounges
	lounges, err := h.loungeRepo.GetLoungesByOwnerID(owner.ID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges for owner %s: %v", owner.ID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	// Convert to response format
	response := make([]gin.H, 0, len(lounges))
	for _, lounge := range lounges {
		// Parse JSONB fields
		var amenities []string
		var images []string

		if lounge.Amenities != nil {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if lounge.Images != nil {
			json.Unmarshal(lounge.Images, &images)
		}

		response = append(response, gin.H{
			"id":               lounge.ID,
			"lounge_name":      lounge.LoungeName,
			"address":          lounge.Address,
			"city":             lounge.City,
			"contact_phone":    lounge.ContactPhone,
			"latitude":         lounge.Latitude,
			"longitude":        lounge.Longitude,
			"price_1_hour":     lounge.Price1Hour,
			"price_2_hours":    lounge.Price2Hours,
			"price_until_bus":  lounge.PriceUntilBus,
			"amenities":        amenities,
			"images":           images,
			"status":           lounge.Status,
			"is_operational":   lounge.IsOperational,
			"total_staff":      lounge.TotalStaff,
			"average_rating":   lounge.AverageRating,
			"total_bookings":   lounge.TotalBookings,
			"created_at":       lounge.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges": response,
		"total":   len(response),
	})
}

// ===================================================================
// GET LOUNGE BY ID
// ===================================================================

// GetLoungeByID handles GET /api/v1/lounges/:id
func (h *LoungeHandler) GetLoungeByID(c *gin.Context) {
	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounge",
		})
		return
	}

	if lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	// Parse JSONB fields
	var amenities []string
	var images []string

	if lounge.Amenities != nil {
		json.Unmarshal(lounge.Amenities, &amenities)
	}
	if lounge.Images != nil {
		json.Unmarshal(lounge.Images, &images)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":               lounge.ID,
		"lounge_owner_id":  lounge.LoungeOwnerID,
		"lounge_name":      lounge.LoungeName,
		"address":          lounge.Address,
		"city":             lounge.City,
		"contact_phone":    lounge.ContactPhone,
		"latitude":         lounge.Latitude,
		"longitude":        lounge.Longitude,
		"price_1_hour":     lounge.Price1Hour,
		"price_2_hours":    lounge.Price2Hours,
		"price_until_bus":  lounge.PriceUntilBus,
		"amenities":        amenities,
		"images":           images,
		"status":           lounge.Status,
		"is_operational":   lounge.IsOperational,
		"total_staff":      lounge.TotalStaff,
		"average_rating":   lounge.AverageRating,
		"total_bookings":   lounge.TotalBookings,
		"created_at":       lounge.CreatedAt,
		"updated_at":       lounge.UpdatedAt,
	})
}

// ===================================================================
// UPDATE LOUNGE
// ===================================================================

// UpdateLoungeRequest represents the lounge update request
type UpdateLoungeRequest struct {
	LoungeName     string   `json:"lounge_name" binding:"required"`
	Address        string   `json:"address" binding:"required"`
	City           string   `json:"city" binding:"required"`
	ContactPhone   string   `json:"contact_phone" binding:"required"`
	Latitude       *string  `json:"latitude"`
	Longitude      *string  `json:"longitude"`
	Price1Hour     *string  `json:"price_1_hour"`
	Price2Hours    *string  `json:"price_2_hours"`
	PriceUntilBus  *string  `json:"price_until_bus"`
	Amenities      []string `json:"amenities"`
	Images         []string `json:"images"`
}

// UpdateLounge handles PUT /api/v1/lounges/:id
func (h *LoungeHandler) UpdateLounge(c *gin.Context) {
	// Get user context from JWT middleware
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

	var req UpdateLoungeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	// Verify ownership
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
			Message: "You don't have permission to update this lounge",
		})
		return
	}

	// Convert amenities and images to JSONB
	amenitiesJSON, _ := json.Marshal(req.Amenities)
	imagesJSON, _ := json.Marshal(req.Images)

	// Update lounge
	err = h.loungeRepo.UpdateLounge(
		loungeID,
		req.LoungeName,
		req.Address,
		req.City,
		req.ContactPhone,
		req.Latitude,
		req.Longitude,
		req.Price1Hour,
		req.Price2Hours,
		req.PriceUntilBus,
		amenitiesJSON,
		imagesJSON,
	)
	if err != nil {
		log.Printf("ERROR: Failed to update lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update lounge",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lounge updated successfully",
	})
}

// ===================================================================
// DELETE LOUNGE
// ===================================================================

// DeleteLounge handles DELETE /api/v1/lounges/:id
func (h *LoungeHandler) DeleteLounge(c *gin.Context) {
	// Get user context from JWT middleware
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

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	// Verify ownership
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
			Message: "You don't have permission to delete this lounge",
		})
		return
	}

	// Delete lounge (triggers will handle counts)
	err = h.loungeRepo.DeleteLounge(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to delete lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete lounge",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lounge deleted successfully",
	})
}

// ===================================================================
// GET LOUNGES BY CITY (PUBLIC)
// ===================================================================

// GetLoungesByCity handles GET /api/v1/lounges/city/:city
func (h *LoungeHandler) GetLoungesByCity(c *gin.Context) {
	city := c.Param("city")

	lounges, err := h.loungeRepo.GetLoungesByCity(city)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges for city %s: %v", city, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	// Convert to response format
	response := make([]gin.H, 0, len(lounges))
	for _, lounge := range lounges {
		// Parse JSONB fields
		var amenities []string
		var images []string

		if lounge.Amenities != nil {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if lounge.Images != nil {
			json.Unmarshal(lounge.Images, &images)
		}

		response = append(response, gin.H{
			"id":               lounge.ID,
			"lounge_name":      lounge.LoungeName,
			"address":          lounge.Address,
			"city":             lounge.City,
			"latitude":         lounge.Latitude,
			"longitude":        lounge.Longitude,
			"price_1_hour":     lounge.Price1Hour,
			"price_2_hours":    lounge.Price2Hours,
			"price_until_bus":  lounge.PriceUntilBus,
			"amenities":        amenities,
			"images":           images,
			"average_rating":   lounge.AverageRating,
			"total_bookings":   lounge.TotalBookings,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges": response,
		"total":   len(response),
		"city":    city,
	})
}
