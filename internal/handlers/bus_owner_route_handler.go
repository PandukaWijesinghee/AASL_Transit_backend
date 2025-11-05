package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/repository"
)

type BusOwnerRouteHandler struct {
	routeRepo *repository.BusOwnerRouteRepository
}

func NewBusOwnerRouteHandler(routeRepo *repository.BusOwnerRouteRepository) *BusOwnerRouteHandler {
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

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format"})
		return
	}

	// Validate that all stops exist in the master route
	stopsExist, err := h.routeRepo.ValidateStopsExist(req.MasterRouteID, req.SelectedStopIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate stops"})
		return
	}

	if !stopsExist {
		c.JSON(http.StatusBadRequest, gin.H{"error": "One or more selected stops do not exist in the master route"})
		return
	}

	// Validate that first and last stops are included
	hasFirstAndLast, err := h.routeRepo.ValidateFirstAndLastStops(req.MasterRouteID, req.SelectedStopIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate first and last stops"})
		return
	}

	if !hasFirstAndLast {
		c.JSON(http.StatusBadRequest, gin.H{"error": "First and last stops of the route must be included"})
		return
	}

	// TODO: Verify that user owns a permit for this master route

	// Create route
	route := &models.BusOwnerRoute{
		ID:              uuid.New().String(),
		BusOwnerID:      userCtx.UserID.String(),
		MasterRouteID:   req.MasterRouteID,
		CustomRouteName: req.CustomRouteName,
		Direction:       req.Direction,
		SelectedStopIDs: req.SelectedStopIDs,
	}

	if err := h.routeRepo.Create(route); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create route"})
		return
	}

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

	c.JSON(http.StatusOK, routes)
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

	c.JSON(http.StatusOK, routes)
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
