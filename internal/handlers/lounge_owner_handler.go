package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeOwnerHandler handles lounge owner-related HTTP requests
type LoungeOwnerHandler struct {
	loungeOwnerRepo *database.LoungeOwnerRepository
	loungeRepo      *database.LoungeRepository
	userRepo        *database.UserRepository
}

// NewLoungeOwnerHandler creates a new lounge owner handler
func NewLoungeOwnerHandler(
	loungeOwnerRepo *database.LoungeOwnerRepository,
	loungeRepo *database.LoungeRepository,
	userRepo *database.UserRepository,
) *LoungeOwnerHandler {
	return &LoungeOwnerHandler{
		loungeOwnerRepo: loungeOwnerRepo,
		loungeRepo:      loungeRepo,
		userRepo:        userRepo,
	}
}

// ===================================================================
// STEP 1: Save Personal Information
// ===================================================================

// SavePersonalInfoRequest represents the personal info request
type SavePersonalInfoRequest struct {
	FullName  string  `json:"full_name" binding:"required"`
	NICNumber string  `json:"nic_number" binding:"required"`
	Email     *string `json:"email"` // optional
}

// SavePersonalInfo handles POST /api/v1/lounge-owner/register/personal-info
func (h *LoungeOwnerHandler) SavePersonalInfo(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req SavePersonalInfoRequest
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

	// Update personal info
	err = h.loungeOwnerRepo.UpdatePersonalInfo(
		userCtx.UserID,
		req.FullName,
		req.NICNumber,
		req.Email,
	)
	if err != nil {
		log.Printf("ERROR: Failed to update personal info for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to save personal information",
		})
		return
	}

	log.Printf("INFO: Personal info saved for lounge owner %s (step: personal_info)", userCtx.UserID)

	c.JSON(http.StatusOK, gin.H{
		"message":           "Personal information saved successfully",
		"registration_step": models.RegStepPersonalInfo,
	})
}

// ===================================================================
// STEP 2: Upload NIC Images
// ===================================================================

// UploadNICRequest represents the NIC upload request
type UploadNICRequest struct {
	NICNumber    string `json:"nic_number" binding:"required"`    // For validation
	NICFrontURL  string `json:"nic_front_url" binding:"required"` // Uploaded by client to Supabase
	NICBackURL   string `json:"nic_back_url" binding:"required"`  // Uploaded by client to Supabase
	OCRExtracted string `json:"ocr_extracted" binding:"required"` // NIC number extracted by OCR
	OCRMatched   bool   `json:"ocr_matched" binding:"required"`   // Did OCR match user input?
}

// UploadNIC handles POST /api/v1/lounge-owner/register/upload-nic
func (h *LoungeOwnerHandler) UploadNIC(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req UploadNICRequest
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

	// Check if OCR is blocked
	isBlocked, blockedUntil, err := h.loungeOwnerRepo.IsOCRBlocked(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to check OCR block status: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "check_failed",
			Message: "Failed to check OCR status",
		})
		return
	}

	if isBlocked {
		retryAfter := int(time.Until(blockedUntil).Seconds())
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":         "ocr_blocked",
			"message":       "Too many OCR attempts. Please contact support or try again after 24 hours.",
			"retry_after":   retryAfter,
			"blocked_until": blockedUntil.Format(time.RFC3339),
		})
		return
	}

	// Check if NIC number from database matches the one being uploaded
	if owner.NICNumber.Valid && owner.NICNumber.String != req.NICNumber {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "nic_mismatch",
			Message: "NIC number does not match the one provided in personal information",
		})
		return
	}

	// Increment OCR attempts
	err = h.loungeOwnerRepo.IncrementOCRAttempts(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to increment OCR attempts: %v", err)
		// Continue anyway
	}

	// Get current OCR attempts
	attempts, err := h.loungeOwnerRepo.GetOCRAttempts(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get OCR attempts: %v", err)
		attempts = 0
	}

	// Check if OCR matched
	if !req.OCRMatched {
		log.Printf("WARN: OCR did not match for user %s (attempt %d/%d)", userCtx.UserID, attempts, models.MaxOCRAttempts)

		// Check if max attempts reached
		if attempts >= models.MaxOCRAttempts {
			// Block OCR for 24 hours
			err = h.loungeOwnerRepo.SetOCRBlock(userCtx.UserID)
			if err != nil {
				log.Printf("ERROR: Failed to set OCR block: %v", err)
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "max_ocr_attempts",
				"message":         "Maximum OCR attempts reached. Please contact support or try again after 24 hours.",
				"attempts_used":   attempts,
				"max_attempts":    models.MaxOCRAttempts,
				"contact_support": true,
			})
			return
		}

		// Return error with remaining attempts
		remainingAttempts := models.MaxOCRAttempts - attempts
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "ocr_mismatch",
			"message":            "NIC number extracted from image does not match your input. Please retake the photo.",
			"attempts_used":      attempts,
			"remaining_attempts": remainingAttempts,
			"ocr_extracted":      req.OCRExtracted,
			"user_input":         req.NICNumber,
		})
		return
	}

	// OCR matched! Save NIC images
	err = h.loungeOwnerRepo.UpdateNICImages(
		userCtx.UserID,
		req.NICFrontURL,
		req.NICBackURL,
	)
	if err != nil {
		log.Printf("ERROR: Failed to update NIC images for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to save NIC images",
		})
		return
	}

	log.Printf("INFO: NIC images uploaded successfully for lounge owner %s (step: nic_uploaded)", userCtx.UserID)

	c.JSON(http.StatusOK, gin.H{
		"message":           "NIC images uploaded successfully",
		"registration_step": models.RegStepNICUploaded,
		"attempts_used":     attempts,
	})
}

// ===================================================================
// STEP 3: Add Lounge Details
// ===================================================================

// AddLoungeRequest represents the lounge creation request
type AddLoungeRequest struct {
	LoungeName        string                 `json:"lounge_name" binding:"required"`
	BusinessLicense   *string                `json:"business_license"`
	FullAddress       string                 `json:"full_address" binding:"required"`
	City              string                 `json:"city" binding:"required"`
	State             *string                `json:"state"`
	PostalCode        *string                `json:"postal_code"`
	Latitude          *string                `json:"latitude"`
	Longitude         *string                `json:"longitude"`
	ContactPersonName *string                `json:"contact_person_name"`
	BusinessEmail     *string                `json:"business_email"`
	BusinessPhone     *string                `json:"business_phone"`
	LoungePhotos      []models.LoungePhoto   `json:"lounge_photos" binding:"required,min=1,max=5"` // 1-5 photos
	Facilities        []string               `json:"facilities"`                                   // Array of facility names
	OperatingHours    *models.OperatingHours `json:"operating_hours"`
	Description       *string                `json:"description"`
}

// AddLounge handles POST /api/v1/lounge-owner/register/add-lounge
func (h *LoungeOwnerHandler) AddLounge(c *gin.Context) {
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

	// Validate lounge photos count (1-5)
	if len(req.LoungePhotos) < 1 || len(req.LoungePhotos) > 5 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Lounge photos must be between 1 and 5",
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

	// Check if previous steps are completed
	if owner.RegistrationStep != models.RegStepNICUploaded && owner.RegistrationStep != models.RegStepCompleted {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "incomplete_registration",
			Message: "Please complete previous registration steps first",
		})
		return
	}

	// Create lounge
	lounge, err := h.loungeRepo.CreateLounge(database.CreateLoungeRequest{
		LoungeOwnerID:     owner.ID,
		LoungeName:        req.LoungeName,
		BusinessLicense:   req.BusinessLicense,
		FullAddress:       req.FullAddress,
		City:              req.City,
		State:             req.State,
		PostalCode:        req.PostalCode,
		Latitude:          req.Latitude,
		Longitude:         req.Longitude,
		ContactPersonName: req.ContactPersonName,
		BusinessEmail:     req.BusinessEmail,
		BusinessPhone:     req.BusinessPhone,
		LoungePhotos:      req.LoungePhotos,
		Facilities:        req.Facilities,
		OperatingHours:    req.OperatingHours,
		Description:       req.Description,
	})
	if err != nil {
		log.Printf("ERROR: Failed to create lounge for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "creation_failed",
			Message: "Failed to create lounge",
		})
		return
	}

	// Update registration step to lounge_added
	err = h.loungeOwnerRepo.UpdateRegistrationStep(userCtx.UserID, models.RegStepLoungeAdded)
	if err != nil {
		log.Printf("ERROR: Failed to update registration step: %v", err)
		// Continue anyway
	}

	// Complete registration
	err = h.loungeOwnerRepo.CompleteRegistration(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to complete registration: %v", err)
		// Continue anyway
	}

	// Increment total lounges count
	err = h.loungeOwnerRepo.IncrementTotalLounges(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to increment total lounges: %v", err)
		// Continue anyway
	}

	log.Printf("INFO: Lounge created successfully for lounge owner %s (lounge_id: %s, step: completed)", userCtx.UserID, lounge.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":             "Lounge registered successfully",
		"lounge_id":           lounge.ID,
		"registration_step":   models.RegStepCompleted,
		"verification_status": lounge.VerificationStatus,
	})
}

// ===================================================================
// GET REGISTRATION PROGRESS
// ===================================================================

// GetRegistrationProgress handles GET /api/v1/lounge-owner/registration/progress
func (h *LoungeOwnerHandler) GetRegistrationProgress(c *gin.Context) {
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
			Message: "Failed to retrieve registration progress",
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

	// Check OCR block status
	isOCRBlocked, blockedUntil, _ := h.loungeOwnerRepo.IsOCRBlocked(userCtx.UserID)
	ocrAttempts, _ := h.loungeOwnerRepo.GetOCRAttempts(userCtx.UserID)

	response := gin.H{
		"registration_step":   owner.RegistrationStep,
		"profile_completed":   owner.ProfileCompleted,
		"verification_status": owner.VerificationStatus,
		"ocr_attempts":        ocrAttempts,
		"ocr_blocked":         isOCRBlocked,
		"total_lounges":       owner.TotalLounges,
	}

	if isOCRBlocked {
		response["ocr_blocked_until"] = blockedUntil.Format(time.RFC3339)
		response["retry_after_seconds"] = int(time.Until(blockedUntil).Seconds())
	}

	// Add step completion status
	response["steps"] = gin.H{
		"phone_verified": true, // Always true if they have a record
		"personal_info":  owner.RegistrationStep == models.RegStepPersonalInfo || owner.RegistrationStep == models.RegStepNICUploaded || owner.RegistrationStep == models.RegStepLoungeAdded || owner.RegistrationStep == models.RegStepCompleted,
		"nic_uploaded":   owner.RegistrationStep == models.RegStepNICUploaded || owner.RegistrationStep == models.RegStepLoungeAdded || owner.RegistrationStep == models.RegStepCompleted,
		"lounge_added":   owner.RegistrationStep == models.RegStepLoungeAdded || owner.RegistrationStep == models.RegStepCompleted,
		"completed":      owner.RegistrationStep == models.RegStepCompleted,
	}

	c.JSON(http.StatusOK, response)
}

// ===================================================================
// GET LOUNGE OWNER PROFILE
// ===================================================================

// GetProfile handles GET /api/v1/lounge-owner/profile
func (h *LoungeOwnerHandler) GetProfile(c *gin.Context) {
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
			Message: "Failed to retrieve profile",
		})
		return
	}

	if owner == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge owner profile not found",
		})
		return
	}

	// Get lounges
	lounges, err := h.loungeRepo.GetLoungesByOwnerID(owner.ID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges for owner %s: %v", owner.ID, err)
		lounges = []*models.Lounge{} // Return empty array on error
	}

	// Convert lounges to response format
	loungeResponses := make([]gin.H, 0, len(lounges))
	for _, lounge := range lounges {
		// Parse JSON fields
		var photos []models.LoungePhoto
		var facilities []string
		var operatingHours *models.OperatingHours

		if lounge.LoungePhotos != nil {
			json.Unmarshal(lounge.LoungePhotos, &photos)
		}
		if lounge.Facilities != nil {
			json.Unmarshal(lounge.Facilities, &facilities)
		}
		if lounge.OperatingHours != nil {
			json.Unmarshal(lounge.OperatingHours, &operatingHours)
		}

		loungeResponses = append(loungeResponses, gin.H{
			"id":                  lounge.ID,
			"lounge_name":         lounge.LoungeName,
			"city":                lounge.City,
			"full_address":        lounge.FullAddress,
			"verification_status": lounge.VerificationStatus,
			"lounge_photos":       photos,
			"facilities":          facilities,
			"operating_hours":     operatingHours,
			"is_active":           lounge.IsActive,
			"created_at":          lounge.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                  owner.ID,
		"user_id":             owner.UserID,
		"full_name":           owner.FullName,
		"nic_number":          owner.NICNumber,
		"business_email":      owner.BusinessEmail,
		"registration_step":   owner.RegistrationStep,
		"profile_completed":   owner.ProfileCompleted,
		"verification_status": owner.VerificationStatus,
		"total_lounges":       owner.TotalLounges,
		"lounges":             loungeResponses,
		"created_at":          owner.CreatedAt,
	})
}

// ===================================================================
// GET MY LOUNGES
// ===================================================================

// GetMyLounges handles GET /api/v1/lounge-owner/lounges
func (h *LoungeOwnerHandler) GetMyLounges(c *gin.Context) {
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
		// Parse JSON fields
		var photos []models.LoungePhoto
		var facilities []string

		if lounge.LoungePhotos != nil {
			json.Unmarshal(lounge.LoungePhotos, &photos)
		}
		if lounge.Facilities != nil {
			json.Unmarshal(lounge.Facilities, &facilities)
		}

		response = append(response, gin.H{
			"id":                  lounge.ID,
			"lounge_name":         lounge.LoungeName,
			"city":                lounge.City,
			"full_address":        lounge.FullAddress,
			"verification_status": lounge.VerificationStatus,
			"lounge_photos":       photos,
			"facilities":          facilities,
			"is_active":           lounge.IsActive,
			"created_at":          lounge.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges": response,
		"total":   len(response),
	})
}
