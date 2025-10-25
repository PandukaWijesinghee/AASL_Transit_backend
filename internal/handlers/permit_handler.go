package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type PermitHandler struct {
	permitRepo   *database.RoutePermitRepository
	busOwnerRepo *database.BusOwnerRepository
}

func NewPermitHandler(permitRepo *database.RoutePermitRepository, busOwnerRepo *database.BusOwnerRepository) *PermitHandler {
	return &PermitHandler{
		permitRepo:   permitRepo,
		busOwnerRepo: busOwnerRepo,
	}
}

// GetAllPermits retrieves all permits for the authenticated bus owner
// GET /api/v1/permits
func (h *PermitHandler) GetAllPermits(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Get permits
	permits, err := h.permitRepo.GetByOwnerID(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permits"})
		return
	}

	c.JSON(http.StatusOK, permits)
}

// GetValidPermits retrieves only valid (non-expired) permits
// GET /api/v1/permits/valid
func (h *PermitHandler) GetValidPermits(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Get valid permits
	permits, err := h.permitRepo.GetValidPermits(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch valid permits"})
		return
	}

	c.JSON(http.StatusOK, permits)
}

// GetPermitByID retrieves a specific permit by ID
// GET /api/v1/permits/:id
func (h *PermitHandler) GetPermitByID(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Get permit ID from URL
	permitID := c.Param("id")

	// Get permit
	permit, err := h.permitRepo.GetByID(permitID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Permit not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permit"})
		return
	}

	// Verify ownership
	if permit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, permit)
}

// CreatePermit creates a new route permit
// POST /api/v1/permits
func (h *PermitHandler) CreatePermit(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Parse request
	var req models.CreateRoutePermitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create permit model from request
	permit, err := models.NewRoutePermitFromRequest(busOwner.ID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save to database
	err = h.permitRepo.Create(permit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create permit: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, permit)
}

// UpdatePermit updates an existing permit
// PUT /api/v1/permits/:id
func (h *PermitHandler) UpdatePermit(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Get permit ID from URL
	permitID := c.Param("id")

	// Verify ownership first
	existingPermit, err := h.permitRepo.GetByID(permitID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Permit not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permit"})
		return
	}

	if existingPermit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Parse update request
	var req models.UpdateRoutePermitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update permit
	err = h.permitRepo.Update(permitID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permit: " + err.Error()})
		return
	}

	// Fetch updated permit
	updatedPermit, err := h.permitRepo.GetByID(permitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated permit"})
		return
	}

	c.JSON(http.StatusOK, updatedPermit)
}

// DeletePermit deletes a permit
// DELETE /api/v1/permits/:id
func (h *PermitHandler) DeletePermit(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Get permit ID from URL
	permitID := c.Param("id")

	// Delete permit (repository will verify ownership)
	err = h.permitRepo.Delete(permitID, busOwner.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Permit not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete permit: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permit deleted successfully"})
}
