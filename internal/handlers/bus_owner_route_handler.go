package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type BusOwnerRouteHandler struct {
	routeRepo *database.BusOwnerRouteRepository
}

func NewBusOwnerRouteHandler(routeRepo *database.BusOwnerRouteRepository) *BusOwnerRouteHandler {
	return &BusOwnerRouteHandler{
		routeRepo: routeRepo,
	}
}

// CreateRoute creates a new custom route
// POST /api/v1/bus-owner-routes
func (h *BusOwnerRouteHandler) CreateRoute(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateBusOwnerRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the request for debugging
	log.Printf("🚌 [BUS OWNER ROUTE] CreateRoute - BusOwner: %s, MasterRoute: %s, Name: %s, Direction: %s, Stops: %d",
		userCtx.UserID.String(), req.MasterRouteID, req.CustomRouteName, req.Direction, len(req.SelectedStopIDs))

	// Validate request
	if err := req.Validate(); err != nil {
		log.Printf("❌ [BUS OWNER ROUTE] Validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format", "details": err.Error()})
		return
	}

	// Validate that all stops exist in the master route
	log.Printf("🔍 [BUS OWNER ROUTE] Validating stops exist for master route: %s", req.MasterRouteID)
	stopsExist, err := h.routeRepo.ValidateStopsExist(req.MasterRouteID, req.SelectedStopIDs)
	if err != nil {
		log.Printf("❌ [BUS OWNER ROUTE] Stop validation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to validate stops",
			"details": err.Error(),
		})
		return
	}

	if !stopsExist {
		log.Printf("⚠️ [BUS OWNER ROUTE] Some stops don't exist in master route")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "One or more selected stops do not exist in the master route",
			"details": "Please ensure the master route has stops configured",
		})
		return
	}
	log.Printf("✅ [BUS OWNER ROUTE] All stops validated successfully")

	// Validate that first and last stops are included
	log.Printf("🔍 [BUS OWNER ROUTE] Validating first and last stops")
	hasFirstAndLast, err := h.routeRepo.ValidateFirstAndLastStops(req.MasterRouteID, req.SelectedStopIDs)
	if err != nil {
		log.Printf("❌ [BUS OWNER ROUTE] First/last stop validation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to validate first and last stops",
			"details": err.Error(),
		})
		return
	}

	if !hasFirstAndLast {
		log.Printf("⚠️ [BUS OWNER ROUTE] First or last stop missing")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "First and last stops of the route must be included",
			"details": "The origin and destination stops are required",
		})
		return
	}
	log.Printf("✅ [BUS OWNER ROUTE] First and last stops validated successfully")

	// TODO: Verify that user owns a permit for this master route

	// Create route
	log.Printf("💾 [BUS OWNER ROUTE] Creating route in database...")
	route := &models.BusOwnerRoute{
		ID:              uuid.New().String(),
		BusOwnerID:      userCtx.UserID.String(),
		MasterRouteID:   req.MasterRouteID,
		CustomRouteName: req.CustomRouteName,
		Direction:       req.Direction,
		SelectedStopIDs: req.SelectedStopIDs,
	}

	if err := h.routeRepo.Create(route); err != nil {
		log.Printf("❌ [BUS OWNER ROUTE] Failed to create route: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create route",
			"details": err.Error(),
		})
		return
	}

	log.Printf("✅ [BUS OWNER ROUTE] Route created successfully: %s", route.ID)
	c.JSON(http.StatusCreated, route)
}

// GetRoutes retrieves all custom routes for the authenticated bus owner
// GET /api/v1/bus-owner-routes
func (h *BusOwnerRouteHandler) GetRoutes(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	routes, err := h.routeRepo.GetByBusOwnerID(userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch routes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"routes": routes,
		"count":  len(routes),
	})
}

// GetRouteByID retrieves a specific custom route
// GET /api/v1/bus-owner-routes/:id
func (h *BusOwnerRouteHandler) GetRouteByID(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	routeID := c.Param("id")

	route, err := h.routeRepo.GetByID(routeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
		return
	}

	// Verify ownership
	if route.BusOwnerID != userCtx.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, route)
}

// GetRoutesByMasterRoute retrieves custom routes for a specific master route
// GET /api/v1/bus-owner-routes/by-master-route/:master_route_id
func (h *BusOwnerRouteHandler) GetRoutesByMasterRoute(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	masterRouteID := c.Param("master_route_id")

	routes, err := h.routeRepo.GetByMasterRouteID(userCtx.UserID.String(), masterRouteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch routes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"routes": routes,
		"count":  len(routes),
	})
}

// UpdateRoute updates an existing custom route
// PUT /api/v1/bus-owner-routes/:id
func (h *BusOwnerRouteHandler) UpdateRoute(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	routeID := c.Param("id")

	var req models.UpdateBusOwnerRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing route
	existingRoute, err := h.routeRepo.GetByID(routeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
		return
	}

	// Verify ownership
	if existingRoute.BusOwnerID != userCtx.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Update fields
	if req.CustomRouteName != "" {
		existingRoute.CustomRouteName = req.CustomRouteName
	}

	if len(req.SelectedStopIDs) > 0 {
		// Validate stops
		stopsExist, err := h.routeRepo.ValidateStopsExist(existingRoute.MasterRouteID, req.SelectedStopIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate stops"})
			return
		}

		if !stopsExist {
			c.JSON(http.StatusBadRequest, gin.H{"error": "One or more selected stops do not exist in the master route"})
			return
		}

		// Validate first and last stops
		hasFirstAndLast, err := h.routeRepo.ValidateFirstAndLastStops(existingRoute.MasterRouteID, req.SelectedStopIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate first and last stops"})
			return
		}

		if !hasFirstAndLast {
			c.JSON(http.StatusBadRequest, gin.H{"error": "First and last stops of the route must be included"})
			return
		}

		existingRoute.SelectedStopIDs = req.SelectedStopIDs
	}

	if err := h.routeRepo.Update(existingRoute); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update route"})
		return
	}

	c.JSON(http.StatusOK, existingRoute)
}

// DeleteRoute deletes a custom route
// DELETE /api/v1/bus-owner-routes/:id
func (h *BusOwnerRouteHandler) DeleteRoute(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	routeID := c.Param("id")

	if err := h.routeRepo.Delete(routeID, userCtx.UserID.String()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete route"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted successfully"})
}
