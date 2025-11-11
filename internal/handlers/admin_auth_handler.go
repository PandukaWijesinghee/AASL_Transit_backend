package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// AdminAuthHandler handles admin authentication HTTP requests
type AdminAuthHandler struct {
	adminAuthService *services.AdminAuthService
	logger           *logrus.Logger
}

// NewAdminAuthHandler creates a new admin auth handler
func NewAdminAuthHandler(adminAuthService *services.AdminAuthService, logger *logrus.Logger) *AdminAuthHandler {
	return &AdminAuthHandler{
		adminAuthService: adminAuthService,
		logger:           logger,
	}
}

// Login handles admin login requests
// @Summary Admin login
// @Description Authenticate admin user and return access and refresh tokens
// @Tags Admin Auth
// @Accept json
// @Produce json
// @Param loginRequest body models.AdminLoginRequest true "Login credentials"
// @Success 200 {object} models.AdminLoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /admin/auth/login [post]
func (h *AdminAuthHandler) Login(c *gin.Context) {
	var req models.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	response, err := h.adminAuthService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"email": req.Email,
			"error": err.Error(),
		}).Warn("Admin login failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"admin_id": response.AdminUser.ID,
		"email":    response.AdminUser.Email,
	}).Info("Admin login successful")

	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh requests
// @Summary Refresh access token
// @Description Generate a new access token using a refresh token
// @Tags Admin Auth
// @Accept json
// @Produce json
// @Param refreshRequest body models.AdminRefreshRequest true "Refresh token"
// @Success 200 {object} models.AdminLoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /admin/auth/refresh [post]
func (h *AdminAuthHandler) RefreshToken(c *gin.Context) {
	var req models.AdminRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	response, err := h.adminAuthService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		h.logger.WithError(err).Warn("Token refresh failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Logout handles admin logout requests
// @Summary Admin logout
// @Description Revoke the refresh token
// @Tags Admin Auth
// @Accept json
// @Produce json
// @Param refreshRequest body models.AdminRefreshRequest true "Refresh token"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /admin/auth/logout [post]
func (h *AdminAuthHandler) Logout(c *gin.Context) {
	var req models.AdminRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if err := h.adminAuthService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		h.logger.WithError(err).Warn("Logout failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetProfile retrieves the current admin's profile
// @Summary Get admin profile
// @Description Get the authenticated admin user's profile
// @Tags Admin Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.AdminUser
// @Failure 401 {object} ErrorResponse
// @Router /admin/auth/profile [get]
func (h *AdminAuthHandler) GetProfile(c *gin.Context) {
	// Get admin ID from context (set by auth middleware)
	adminID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	adminUUID, err := uuid.Parse(adminID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid admin ID"})
		return
	}

	admin, err := h.adminAuthService.GetAdminProfile(c.Request.Context(), adminUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Admin not found"})
		return
	}

	c.JSON(http.StatusOK, admin)
}

// ChangePassword handles password change requests
// @Summary Change admin password
// @Description Change the authenticated admin user's password
// @Tags Admin Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param changePasswordRequest body models.AdminChangePasswordRequest true "Password change request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /admin/auth/change-password [post]
func (h *AdminAuthHandler) ChangePassword(c *gin.Context) {
	// Get admin ID from context
	adminID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	adminUUID, err := uuid.Parse(adminID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid admin ID"})
		return
	}

	var req models.AdminChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if err := h.adminAuthService.ChangePassword(c.Request.Context(), adminUUID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.WithField("admin_id", adminUUID).Info("Admin password changed")
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// CreateAdmin creates a new admin user (only accessible by existing admins)
// @Summary Create new admin user
// @Description Create a new admin user (requires admin authentication)
// @Tags Admin Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param createRequest body models.AdminCreateRequest true "Admin creation request"
// @Success 201 {object} models.AdminUser
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /admin/auth/create [post]
func (h *AdminAuthHandler) CreateAdmin(c *gin.Context) {
	// Get creator admin ID from context
	creatorID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	creatorUUID, err := uuid.Parse(creatorID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid creator ID"})
		return
	}

	var req models.AdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	admin, err := h.adminAuthService.CreateAdmin(c.Request.Context(), req.Email, req.Password, req.FullName, creatorUUID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"admin_id":   admin.ID,
		"email":      admin.Email,
		"created_by": creatorUUID,
	}).Info("New admin user created")

	c.JSON(http.StatusCreated, admin)
}

// ListAdmins retrieves all admin users
// @Summary List all admin users
// @Description Get a list of all admin users (requires admin authentication)
// @Tags Admin Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.AdminUser
// @Failure 401 {object} ErrorResponse
// @Router /admin/auth/list [get]
func (h *AdminAuthHandler) ListAdmins(c *gin.Context) {
	admins, err := h.adminAuthService.ListAdmins(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve admin users"})
		return
	}

	c.JSON(http.StatusOK, admins)
}
