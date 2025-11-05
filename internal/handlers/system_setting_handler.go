package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type SystemSettingHandler struct {
	settingRepo *database.SystemSettingRepository
}

func NewSystemSettingHandler(settingRepo *database.SystemSettingRepository) *SystemSettingHandler {
	return &SystemSettingHandler{
		settingRepo: settingRepo,
	}
}

// GetAllSettings retrieves all system settings
// GET /api/v1/system-settings
func (h *SystemSettingHandler) GetAllSettings(c *gin.Context) {
	settings, err := h.settingRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// GetSettingByKey retrieves a specific system setting by key
// GET /api/v1/system-settings/:key
func (h *SystemSettingHandler) GetSettingByKey(c *gin.Context) {
	key := c.Param("key")

	setting, err := h.settingRepo.GetByKey(key)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Setting not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch setting"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

// UpdateSetting updates a system setting's value
// PUT /api/v1/system-settings/:key
// NOTE: In production, this should be admin-only
func (h *SystemSettingHandler) UpdateSetting(c *gin.Context) {
	key := c.Param("key")

	var req models.UpdateSystemSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Verify setting exists
	_, err := h.settingRepo.GetByKey(key)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Setting not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch setting"})
		return
	}

	// Update setting
	if err := h.settingRepo.Update(key, req.SettingValue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
		return
	}

	// Fetch updated setting
	updatedSetting, err := h.settingRepo.GetByKey(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated setting"})
		return
	}

	c.JSON(http.StatusOK, updatedSetting)
}
