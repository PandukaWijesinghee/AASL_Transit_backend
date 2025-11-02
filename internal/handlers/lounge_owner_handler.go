package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeOwnerHandler handles lounge owner-related HTTP requests
type LoungeOwnerHandler struct {
	loungeOwnerRepo *database.LoungeOwnerRepository
	userRepo        *database.UserRepository
}

// NewLoungeOwnerHandler creates a new lounge owner handler
func NewLoungeOwnerHandler(
	loungeOwnerRepo *database.LoungeOwnerRepository,
	userRepo *database.UserRepository,
) *LoungeOwnerHandler {
	return &LoungeOwnerHandler{
		loungeOwnerRepo: loungeOwnerRepo,
		userRepo:        userRepo,
	}
}

// ===================================================================
// STEP 1: Save Business and Manager Information
// ===================================================================

// SaveBusinessAndManagerInfoRequest represents the business/manager info request
type SaveBusinessAndManagerInfoRequest struct {
	BusinessName     string  `json:"business_name" binding:"required"`
	BusinessLicense  *string `json:"business_license"`
	ManagerFullName  string  `json:"manager_full_name" binding:"required"`
	ManagerNICNumber string  `json:"manager_nic_number" binding:"required"`
	ManagerEmail     *string `json:"manager_email"`
}

// SaveBusinessAndManagerInfo handles POST /api/v1/lounge-owner/register/business-info
func (h *LoungeOwnerHandler) SaveBusinessAndManagerInfo(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req SaveBusinessAndManagerInfoRequest
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

	// Update business and manager info
	businessLicenseVal := ""
	if req.BusinessLicense != nil {
		businessLicenseVal = *req.BusinessLicense
	}
	err = h.loungeOwnerRepo.UpdateBusinessAndManagerInfo(
		userCtx.UserID,
		req.BusinessName,
		businessLicenseVal,
		req.ManagerFullName,
		req.ManagerNICNumber,
		req.ManagerEmail,
	)
	if err != nil {
		log.Printf("ERROR: Failed to update business/manager info for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to save business and manager information",
		})
		return
	}

	log.Printf("INFO: Business and manager info saved for lounge owner %s (step: business_info)", userCtx.UserID)

	c.JSON(http.StatusOK, gin.H{
		"message":           "Business and manager information saved successfully",
		"registration_step": models.RegStepBusinessInfo,
	})
}

// ===================================================================
// STEP 2: Upload Manager NIC Images
// ===================================================================

// UploadManagerNICRequest represents the manager NIC upload request
type UploadManagerNICRequest struct {
	ManagerNICFrontURL string `json:"manager_nic_front_url" binding:"required"` // Uploaded to Supabase
	ManagerNICBackURL  string `json:"manager_nic_back_url" binding:"required"`  // Uploaded to Supabase
}

// UploadManagerNIC handles POST /api/v1/lounge-owner/register/upload-manager-nic
func (h *LoungeOwnerHandler) UploadManagerNIC(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req UploadManagerNICRequest
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

	// Check if previous step (business_info) is completed
	if owner.RegistrationStep != models.RegStepBusinessInfo {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "incomplete_registration",
			Message: "Please complete business information step first",
		})
		return
	}

	// Save manager NIC images
	err = h.loungeOwnerRepo.UpdateManagerNICImages(
		userCtx.UserID,
		req.ManagerNICFrontURL,
		req.ManagerNICBackURL,
	)
	if err != nil {
		log.Printf("ERROR: Failed to update manager NIC images for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to save manager NIC images",
		})
		return
	}

	log.Printf("INFO: Manager NIC images uploaded successfully for lounge owner %s (step: nic_uploaded)", userCtx.UserID)

	c.JSON(http.StatusOK, gin.H{
		"message":           "Manager NIC images uploaded successfully",
		"registration_step": models.RegStepNICUploaded,
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

	response := gin.H{
		"registration_step":   owner.RegistrationStep,
		"profile_completed":   owner.ProfileCompleted,
		"verification_status": owner.VerificationStatus,
		"total_lounges":       owner.TotalLounges,
		"total_staff":         owner.TotalStaff,
	}

	// Add step completion status
	response["steps"] = gin.H{
		"phone_verified": true, // Always true if they have a record
		"business_info":  owner.RegistrationStep == models.RegStepBusinessInfo || owner.RegistrationStep == models.RegStepNICUploaded || owner.RegistrationStep == models.RegStepLoungeAdded || owner.RegistrationStep == models.RegStepCompleted,
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

	c.JSON(http.StatusOK, gin.H{
		"id":                   owner.ID,
		"user_id":              owner.UserID,
		"business_name":        owner.BusinessName,
		"business_license":     owner.BusinessLicense,
		"manager_full_name":    owner.ManagerFullName,
		"manager_nic_number":   owner.ManagerNICNumber,
		"manager_email":        owner.ManagerEmail,
		"manager_nic_front":    owner.ManagerNICFrontURL,
		"manager_nic_back":     owner.ManagerNICBackURL,
		"registration_step":    owner.RegistrationStep,
		"profile_completed":    owner.ProfileCompleted,
		"verification_status":  owner.VerificationStatus,
		"total_lounges":        owner.TotalLounges,
		"total_staff":          owner.TotalStaff,
		"created_at":           owner.CreatedAt,
	})
}
