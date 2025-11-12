package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// BusSeatLayoutHandler handles HTTP requests for bus seat layout templates
type BusSeatLayoutHandler struct {
	service *services.BusSeatLayoutService
	logger  *logrus.Logger
}

// NewBusSeatLayoutHandler creates a new bus seat layout handler
func NewBusSeatLayoutHandler(service *services.BusSeatLayoutService, logger *logrus.Logger) *BusSeatLayoutHandler {
	return &BusSeatLayoutHandler{
		service: service,
		logger:  logger,
	}
}

// CreateTemplate creates a new bus seat layout template
func (h *BusSeatLayoutHandler) CreateTemplate(c *gin.Context) {
	var req models.CreateBusSeatLayoutTemplateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get admin ID from context (set by auth middleware)
	adminIDStr, exists := c.Get("user_id")
	if !exists {
		h.logger.Error("Admin ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	adminID, err := uuid.Parse(adminIDStr.(string))
	if err != nil {
		h.logger.Error("Invalid admin ID format", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid admin ID"})
		return
	}

	// Create template
	template, err := h.service.CreateTemplate(c.Request.Context(), &req, adminID)
	if err != nil {
		h.logger.Error("Failed to create template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template", "details": err.Error()})
		return
	}

	h.logger.Info("Bus seat layout template created successfully", "template_id", template.ID, "admin_id", adminID)
	c.JSON(http.StatusCreated, template)
}

// GetTemplate retrieves a specific template by ID
func (h *BusSeatLayoutHandler) GetTemplate(c *gin.Context) {
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error("Invalid template ID", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	template, err := h.service.GetTemplateByID(c.Request.Context(), templateID)
	if err != nil {
		h.logger.Error("Failed to get template", "template_id", templateID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// ListTemplates retrieves all templates
func (h *BusSeatLayoutHandler) ListTemplates(c *gin.Context) {
	activeOnlyStr := c.Query("active_only")
	activeOnly := activeOnlyStr == "true"

	templates, err := h.service.ListTemplates(c.Request.Context(), activeOnly)
	if err != nil {
		h.logger.Error("Failed to list templates", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
	})
}

// UpdateTemplate updates a template's basic information
func (h *BusSeatLayoutHandler) UpdateTemplate(c *gin.Context) {
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error("Invalid template ID", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	var req models.UpdateBusSeatLayoutTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.service.UpdateTemplate(c.Request.Context(), templateID, &req); err != nil {
		h.logger.Error("Failed to update template", "template_id", templateID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
		return
	}

	h.logger.Info("Template updated successfully", "template_id", templateID)
	c.JSON(http.StatusOK, gin.H{"message": "Template updated successfully"})
}

// DeleteTemplate deletes a template
func (h *BusSeatLayoutHandler) DeleteTemplate(c *gin.Context) {
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error("Invalid template ID", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	if err := h.service.DeleteTemplate(c.Request.Context(), templateID); err != nil {
		h.logger.Error("Failed to delete template", "template_id", templateID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	h.logger.Info("Template deleted successfully", "template_id", templateID)
	c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
}
