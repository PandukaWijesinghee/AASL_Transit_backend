package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type BusHandler struct {
	busRepo      *database.BusRepository
	permitRepo   *database.RoutePermitRepository
	busOwnerRepo *database.BusOwnerRepository
}

func NewBusHandler(busRepo *database.BusRepository, permitRepo *database.RoutePermitRepository, busOwnerRepo *database.BusOwnerRepository) *BusHandler {
	return &BusHandler{
		busRepo:      busRepo,
		permitRepo:   permitRepo,
		busOwnerRepo: busOwnerRepo,
	}
}

// checkBusOwnerVerified is a helper that checks if the bus owner is verified.
// Returns the bus owner if verified, or sends an error response and returns nil if not.
func (h *BusHandler) checkBusOwnerVerified(c *gin.Context, busOwner *models.BusOwner) bool {
	if busOwner.VerificationStatus != models.VerificationVerified {
		c.JSON(http.StatusForbidden, gin.H{
			"error":               "Bus owner account is not verified",
			"code":                "ACCOUNT_NOT_VERIFIED",
			"verification_status": busOwner.VerificationStatus,
			"message":             "Your account must be verified by admin before you can perform this operation. Please wait for verification or contact support.",
		})
		return false
	}
	return true
}

// GetAllBuses retrieves all buses for the authenticated bus owner
// GET /api/v1/buses
func (h *BusHandler) GetAllBuses(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner record for this user
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			// Bus owner doesn't exist yet, return empty list
			c.JSON(http.StatusOK, []interface{}{})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Get buses by bus_owner_id (NOT user_id!)
	buses, err := h.busRepo.GetByOwnerID(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch buses"})
		return
	}

	c.JSON(http.StatusOK, buses)
}

// GetBusByID retrieves a specific bus by ID
// GET /api/v1/buses/:id
func (h *BusHandler) GetBusByID(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busID := c.Param("id")

	// Get bus owner record for this user
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Get bus
	bus, err := h.busRepo.GetByID(busID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus"})
		return
	}

	// Verify ownership (compare bus_owner_id with bus_owner_id)
	if bus.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to access this bus"})
		return
	}

	c.JSON(http.StatusOK, bus)
}

// CreateBus creates a new bus
// POST /api/v1/buses
func (h *BusHandler) CreateBus(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateBusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get bus owner record for this user
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found. Please complete onboarding first."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Check if bus owner is verified before allowing bus creation
	if !h.checkBusOwnerVerified(c, busOwner) {
		return
	}

	// Verify permit exists and belongs to this owner
	permit, err := h.permitRepo.GetByID(req.PermitID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Permit not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permit"})
		return
	}

	// Verify permit ownership (compare bus_owner_id with bus_owner_id, NOT user_id!)
	if permit.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permit does not belong to you"})
		return
	}

	// Check if a bus already exists for this permit (1 permit = 1 bus)
	existingBus, err := h.busRepo.GetByPermitID(req.PermitID)
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing bus"})
		return
	}
	if existingBus != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "A bus is already registered under this permit",
			"bus":   existingBus,
		})
		return
	}

	// Parse dates if provided
	var lastMaintenanceDate *time.Time
	if req.LastMaintenanceDate != nil {
		parsed, err := time.Parse("2006-01-02", *req.LastMaintenanceDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid last_maintenance_date format. Use YYYY-MM-DD"})
			return
		}
		lastMaintenanceDate = &parsed
	}

	var insuranceExpiry *time.Time
	if req.InsuranceExpiry != nil {
		parsed, err := time.Parse("2006-01-02", *req.InsuranceExpiry)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid insurance_expiry format. Use YYYY-MM-DD"})
			return
		}
		insuranceExpiry = &parsed
	}

	// Default status to active if not provided
	status := models.BusStatusActive
	if req.Status != nil {
		status = models.BusStatus(*req.Status)
	}

	// Create bus
	bus := &models.Bus{
		ID:                  uuid.New().String(),
		BusOwnerID:          busOwner.ID, // Use bus_owner_id, NOT user_id!
		PermitID:            req.PermitID,
		BusNumber:           req.BusNumber,
		LicensePlate:        permit.BusRegistrationNumber, // Get from permit
		BusType:             models.BusType(req.BusType),
		ManufacturingYear:   req.ManufacturingYear,
		LastMaintenanceDate: lastMaintenanceDate,
		InsuranceExpiry:     insuranceExpiry,
		Status:              status,
		SeatLayoutID:        req.SeatLayoutID,
		HasWifi:             req.HasWifi,
		HasAC:               req.HasAC,
		HasChargingPorts:    req.HasChargingPorts,
		HasEntertainment:    req.HasEntertainment,
		HasRefreshments:     req.HasRefreshments,
	}

	err = h.busRepo.Create(bus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bus: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bus)
}

// UpdateBus updates an existing bus
// PUT /api/v1/buses/:id
func (h *BusHandler) UpdateBus(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busID := c.Param("id")

	var req models.UpdateBusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get bus owner record for this user
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Check if bus owner is verified before allowing bus update
	if !h.checkBusOwnerVerified(c, busOwner) {
		return
	}

	// Verify bus exists and belongs to this owner
	bus, err := h.busRepo.GetByID(busID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bus"})
		return
	}

	// Verify ownership (compare bus_owner_id with bus_owner_id)
	if bus.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this bus"})
		return
	}

	// Update bus
	err = h.busRepo.Update(busID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update bus: " + err.Error()})
		return
	}

	// Fetch updated bus
	updatedBus, err := h.busRepo.GetByID(busID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated bus"})
		return
	}

	c.JSON(http.StatusOK, updatedBus)
}

// DeleteBus deletes a bus
// DELETE /api/v1/buses/:id
func (h *BusHandler) DeleteBus(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busID := c.Param("id")

	// Get bus owner record for this user
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Check if bus owner is verified before allowing bus deletion
	if !h.checkBusOwnerVerified(c, busOwner) {
		return
	}

	// Delete bus (repository verifies ownership using bus_owner_id)
	err = h.busRepo.Delete(busID, busOwner.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus not found or you don't have permission to delete it"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete bus"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bus deleted successfully"})
}

// GetBusesByStatus retrieves buses filtered by status
// GET /api/v1/buses/status/:status
func (h *BusHandler) GetBusesByStatus(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	status := c.Param("status")

	// Validate status
	busStatus := models.BusStatus(status)
	if busStatus != models.BusStatusActive && busStatus != models.BusStatusMaintenance &&
		busStatus != models.BusStatusInactive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be active, maintenance, or inactive"})
		return
	}

	// Get bus owner record for this user
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			// Bus owner doesn't exist yet, return empty list
			c.JSON(http.StatusOK, []interface{}{})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Get buses by status (using bus_owner_id, NOT user_id!)
	buses, err := h.busRepo.GetByStatus(busOwner.ID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch buses"})
		return
	}

	c.JSON(http.StatusOK, buses)
}
