package handlers

import (
	"database/sql"
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
// NIC images are optional - can be uploaded here or later for admin review
type SaveBusinessAndManagerInfoRequest struct {
	BusinessName       string  `json:"business_name" binding:"required"`
	BusinessLicense    *string `json:"business_license"`
	ManagerFullName    string  `json:"manager_full_name" binding:"required"`
	ManagerNICNumber   string  `json:"manager_nic_number" binding:"required"`
	ManagerEmail       *string `json:"manager_email"`
	ManagerNICFrontURL *string `json:"manager_nic_front_url"` // Optional: NIC front image URL from Supabase
	ManagerNICBackURL  *string `json:"manager_nic_back_url"`  // Optional: NIC back image URL from Supabase
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

	// Update business and manager info (including optional NIC images)
	businessLicenseVal := ""
	if req.BusinessLicense != nil {
		businessLicenseVal = *req.BusinessLicense
	}

	err = h.loungeOwnerRepo.UpdateBusinessAndManagerInfoWithNIC(
		userCtx.UserID,
		req.BusinessName,
		businessLicenseVal,
		req.ManagerFullName,
		req.ManagerNICNumber,
		req.ManagerEmail,
		req.ManagerNICFrontURL,
		req.ManagerNICBackURL,
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
// DEPRECATED: STEP 2: Upload Manager NIC Images
// ===================================================================
// This endpoint is DEPRECATED and kept for backward compatibility only.
// NIC images should now be uploaded as part of business_info step.
// This endpoint is no longer used in the new registration flow.

// UploadManagerNICRequest represents the manager NIC upload request
type UploadManagerNICRequest struct {
	ManagerNICFrontURL string `json:"manager_nic_front_url" binding:"required"` // Uploaded to Supabase
	ManagerNICBackURL  string `json:"manager_nic_back_url" binding:"required"`  // Uploaded to Supabase
}

// UploadManagerNIC handles POST /api/v1/lounge-owner/register/upload-manager-nic
// DEPRECATED: Use SaveBusinessAndManagerInfo with NIC URLs instead
func (h *LoungeOwnerHandler) UploadManagerNIC(c *gin.Context) {
	c.JSON(http.StatusGone, ErrorResponse{
		Error:   "deprecated_endpoint",
		Message: "This endpoint is deprecated. Please include NIC images in the business-info step.",
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

	// Get dynamic counts
	loungeCount, _ := h.loungeOwnerRepo.GetLoungeCount(owner.ID)
	staffCount, _ := h.loungeOwnerRepo.GetStaffCount(owner.ID)

	response := gin.H{
		"registration_step":   owner.RegistrationStep,
		"profile_completed":   owner.ProfileCompleted,
		"verification_status": owner.VerificationStatus,
		"total_lounges":       loungeCount,
		"total_staff":         staffCount,
	}

	// Add step completion status (new flow: phone_verified -> business_info -> lounge_added -> completed)
	response["steps"] = gin.H{
		"phone_verified": true, // Always true if they have a record
		"business_info":  owner.RegistrationStep == models.RegStepBusinessInfo || owner.RegistrationStep == models.RegStepLoungeAdded || owner.RegistrationStep == models.RegStepCompleted,
		"lounge_added":   owner.RegistrationStep == models.RegStepLoungeAdded || owner.RegistrationStep == models.RegStepCompleted,
		"completed":      owner.RegistrationStep == models.RegStepCompleted,
	}

	// If completed but pending approval, add pending_approval flag
	if owner.RegistrationStep == models.RegStepCompleted && owner.VerificationStatus == models.LoungeVerificationPending {
		response["pending_approval"] = true
		response["approval_message"] = "Your registration is complete and awaiting admin approval"
	} else if owner.VerificationStatus == models.LoungeVerificationApproved {
		response["approved"] = true
		response["approval_message"] = "Your account has been approved"
	} else if owner.VerificationStatus == models.LoungeVerificationRejected {
		response["rejected"] = true
		response["approval_message"] = "Your account has been rejected"
		if owner.VerificationNotes.Valid {
			response["rejection_reason"] = owner.VerificationNotes.String
		}
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

	// 🔍 DEBUG: Log what database returns
	log.Printf("🔍 GET PROFILE - User: %s, RegistrationStep: %s, ProfileCompleted: %t",
		userCtx.UserID, owner.RegistrationStep, owner.ProfileCompleted)

	// Get dynamic counts
	loungeCount, _ := h.loungeOwnerRepo.GetLoungeCount(owner.ID)
	staffCount, _ := h.loungeOwnerRepo.GetStaffCount(owner.ID)

	// Helper functions to extract values from sql.Null* types
	getNullableString := func(ns sql.NullString) *string {
		if ns.Valid {
			return &ns.String
		}
		return nil
	}

	getNullableTime := func(nt sql.NullTime) *string {
		if nt.Valid {
			timeStr := nt.Time.Format("2006-01-02T15:04:05Z07:00")
			return &timeStr
		}
		return nil
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                  owner.ID,
		"user_id":             owner.UserID,
		"business_name":       getNullableString(owner.BusinessName),
		"business_license":    getNullableString(owner.BusinessLicense),
		"manager_full_name":   getNullableString(owner.ManagerFullName),
		"manager_nic_number":  getNullableString(owner.ManagerNICNumber),
		"manager_email":       getNullableString(owner.ManagerEmail),
		"registration_step":   owner.RegistrationStep,
		"profile_completed":   owner.ProfileCompleted,
		"verification_status": owner.VerificationStatus,
		"verification_notes":  getNullableString(owner.VerificationNotes),
		"verified_at":         getNullableTime(owner.VerifiedAt),
		"total_lounges":       loungeCount,
		"total_staff":         staffCount,
		"created_at":          owner.CreatedAt,
		"updated_at":          owner.UpdatedAt,
	})
}
