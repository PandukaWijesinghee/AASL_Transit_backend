package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
)

// LoungeStaffHandler handles lounge staff-related HTTP requests
type LoungeStaffHandler struct {
	staffRepo       *database.LoungeStaffRepository
	loungeRepo      *database.LoungeRepository
	loungeOwnerRepo *database.LoungeOwnerRepository
}

// NewLoungeStaffHandler creates a new lounge staff handler
func NewLoungeStaffHandler(
	staffRepo *database.LoungeStaffRepository,
	loungeRepo *database.LoungeRepository,
	loungeOwnerRepo *database.LoungeOwnerRepository,
) *LoungeStaffHandler {
	return &LoungeStaffHandler{
		staffRepo:       staffRepo,
		loungeRepo:      loungeRepo,
		loungeOwnerRepo: loungeOwnerRepo,
	}
}

// ===================================================================
// ADD STAFF TO LOUNGE (STEP 4 - Optional)
// ===================================================================

// AddStaffRequest represents the staff creation request
type AddStaffRequest struct {
	LoungeID       string `json:"lounge_id" binding:"required"`
	PhoneNumber    string `json:"phone_number" binding:"required"`
	FullName       string `json:"full_name" binding:"required"`
	NICNumber      string `json:"nic_number" binding:"required"`
	PermissionType string `json:"permission_type" binding:"required,oneof=admin staff"`
}

// AddStaff handles POST /api/v1/lounges/:lounge_id/staff
func (h *LoungeStaffHandler) AddStaff(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req AddStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	loungeID, err := uuid.Parse(req.LoungeID)
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

	// Verify lounge ownership
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
			Message: "You don't have permission to add staff to this lounge",
		})
		return
	}

	// Add staff to lounge
	staff, err := h.staffRepo.AddStaffToLounge(
		loungeID,
		req.PhoneNumber,
		req.FullName,
		req.NICNumber,
		req.PermissionType,
	)
	if err != nil {
		log.Printf("ERROR: Failed to add staff to lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "creation_failed",
			Message: "Failed to add staff: " + err.Error(),
		})
		return
	}

	log.Printf("INFO: Staff added to lounge %s (staff_id: %s)", loungeID, staff.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":         "Staff added successfully",
		"staff_id":        staff.ID,
		"phone_number":    staff.PhoneNumber,
		"permission_type": staff.PermissionType,
		"has_registered":  staff.HasRegistered,
	})
}

// ===================================================================
// GET STAFF BY LOUNGE
// ===================================================================

// GetStaffByLounge handles GET /api/v1/lounges/:lounge_id/staff
func (h *LoungeStaffHandler) GetStaffByLounge(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("lounge_id")
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

	// Verify lounge ownership
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
			Message: "You don't have permission to view staff for this lounge",
		})
		return
	}

	// Get staff
	staff, err := h.staffRepo.GetStaffByLoungeID(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get staff for lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve staff",
		})
		return
	}

	// Convert to response format
	response := make([]gin.H, 0, len(staff))
	for _, s := range staff {
		response = append(response, gin.H{
			"id":                s.ID,
			"phone_number":      s.PhoneNumber,
			"full_name":         s.FullName,
			"nic_number":        s.NICNumber,
			"email":             s.Email,
			"permission_type":   s.PermissionType,
			"employment_status": s.EmploymentStatus,
			"has_registered":    s.HasRegistered,
			"invited_at":        s.InvitedAt,
			"registered_at":     s.RegisteredAt,
			"created_at":        s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"staff": response,
		"total": len(response),
	})
}

// ===================================================================
// UPDATE STAFF PERMISSION
// ===================================================================

// UpdateStaffPermissionRequest represents the permission update request
type UpdateStaffPermissionRequest struct {
	PermissionType string `json:"permission_type" binding:"required,oneof=admin staff"`
}

// UpdateStaffPermission handles PUT /api/v1/lounges/:lounge_id/staff/:staff_id/permission
func (h *LoungeStaffHandler) UpdateStaffPermission(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("lounge_id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	staffIDStr := c.Param("staff_id")
	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid staff ID format",
		})
		return
	}

	var req UpdateStaffPermissionRequest
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

	// Verify lounge ownership
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
			Message: "You don't have permission to update staff for this lounge",
		})
		return
	}

	// Update staff permission
	err = h.staffRepo.UpdateStaffPermission(staffID, req.PermissionType)
	if err != nil {
		log.Printf("ERROR: Failed to update staff permission: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update staff permission",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff permission updated successfully",
	})
}

// ===================================================================
// UPDATE STAFF EMPLOYMENT STATUS
// ===================================================================

// UpdateStaffStatusRequest represents the employment status update request
type UpdateStaffStatusRequest struct {
	EmploymentStatus string `json:"employment_status" binding:"required,oneof=active inactive terminated"`
}

// UpdateStaffStatus handles PUT /api/v1/lounges/:lounge_id/staff/:staff_id/status
func (h *LoungeStaffHandler) UpdateStaffStatus(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("lounge_id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	staffIDStr := c.Param("staff_id")
	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid staff ID format",
		})
		return
	}

	var req UpdateStaffStatusRequest
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

	// Verify lounge ownership
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
			Message: "You don't have permission to update staff for this lounge",
		})
		return
	}

	// Update employment status
	err = h.staffRepo.UpdateStaffEmploymentStatus(staffID, req.EmploymentStatus)
	if err != nil {
		log.Printf("ERROR: Failed to update staff status: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update staff status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff status updated successfully",
	})
}

// ===================================================================
// REMOVE STAFF
// ===================================================================

// RemoveStaff handles DELETE /api/v1/lounges/:lounge_id/staff/:staff_id
func (h *LoungeStaffHandler) RemoveStaff(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("lounge_id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	staffIDStr := c.Param("staff_id")
	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid staff ID format",
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

	// Verify lounge ownership
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
			Message: "You don't have permission to remove staff from this lounge",
		})
		return
	}

	// Remove staff (triggers will handle counts)
	err = h.staffRepo.RemoveStaff(staffID)
	if err != nil {
		log.Printf("ERROR: Failed to remove staff %s: %v", staffID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to remove staff",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff removed successfully",
	})
}

// ===================================================================
// GET MY STAFF PROFILE (For staff members to check their lounge)
// ===================================================================

// GetMyStaffProfile handles GET /api/v1/staff/my-profile
func (h *LoungeStaffHandler) GetMyStaffProfile(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get staff record by user_id
	staff, err := h.staffRepo.GetStaffByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get staff for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve staff profile",
		})
		return
	}

	if staff == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Staff profile not found",
		})
		return
	}

	// Get lounge details
	lounge, err := h.loungeRepo.GetLoungeByID(staff.LoungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge %s: %v", staff.LoungeID, err)
		// Continue without lounge details
	}

	response := gin.H{
		"id":                staff.ID,
		"lounge_id":         staff.LoungeID,
		"phone_number":      staff.PhoneNumber,
		"full_name":         staff.FullName,
		"nic_number":        staff.NICNumber,
		"email":             staff.Email,
		"permission_type":   staff.PermissionType,
		"employment_status": staff.EmploymentStatus,
		"has_registered":    staff.HasRegistered,
		"invited_at":        staff.InvitedAt,
		"registered_at":     staff.RegisteredAt,
	}

	if lounge != nil {
		response["lounge"] = gin.H{
			"id":          lounge.ID,
			"lounge_name": lounge.LoungeName,
			"address":     lounge.Address,
			"city":        lounge.City,
			"status":      lounge.Status,
		}
	}

	c.JSON(http.StatusOK, response)
}
