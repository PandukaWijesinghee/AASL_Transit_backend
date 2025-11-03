package handlers

import (
	"fmt"
	"net/http"

	"github.com/smarttransit/sms-auth-backend/internal/database"

	"github.com/gin-gonic/gin"
)

// MasterRouteHandler handles master routes operations
type MasterRouteHandler struct {
	masterRouteRepo *database.MasterRouteRepository
}

// NewMasterRouteHandler creates a new master route handler
func NewMasterRouteHandler(masterRouteRepo *database.MasterRouteRepository) *MasterRouteHandler {
	return &MasterRouteHandler{
		masterRouteRepo: masterRouteRepo,
	}
}

// ListMasterRoutes returns all active master routes for dropdown selection
// GET /api/v1/master-routes
func (h *MasterRouteHandler) ListMasterRoutes(c *gin.Context) {
	// Get all active master routes
	routes, err := h.masterRouteRepo.GetAll(true) // true = active only
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch master routes"})
		return
	}

	// Format response for dropdown
	type MasterRouteOption struct {
		ID                       string  `json:"id"`
		RouteNumber              string  `json:"route_number"`
		RouteName                string  `json:"route_name"`
		OriginCity               string  `json:"origin_city"`
		DestinationCity          string  `json:"destination_city"`
		TotalDistanceKm          float64 `json:"total_distance_km"`
		EstimatedDurationMinutes int     `json:"estimated_duration_minutes"`
		DisplayLabel             string  `json:"display_label"` // e.g., "99 - Colombo → Galle (116 km)"
	}

	options := make([]MasterRouteOption, 0, len(routes))
	for _, route := range routes {
		displayLabel := route.RouteNumber + " - " + route.OriginCity + " → " + route.DestinationCity
		if route.TotalDistanceKm > 0 {
			displayLabel += " (" + formatDistance(route.TotalDistanceKm) + ")"
		}

		options = append(options, MasterRouteOption{
			ID:                       route.ID,
			RouteNumber:              route.RouteNumber,
			RouteName:                route.RouteName,
			OriginCity:               route.OriginCity,
			DestinationCity:          route.DestinationCity,
			TotalDistanceKm:          route.TotalDistanceKm,
			EstimatedDurationMinutes: route.EstimatedDurationMinutes,
			DisplayLabel:             displayLabel,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"master_routes": options,
		"count":         len(options),
	})
}

// GetMasterRouteByID returns a specific master route with details
// GET /api/v1/master-routes/:id
func (h *MasterRouteHandler) GetMasterRouteByID(c *gin.Context) {
	routeID := c.Param("id")

	route, err := h.masterRouteRepo.GetByID(routeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Master route not found"})
		return
	}

	// Get stops for this route
	stops, err := h.masterRouteRepo.GetStopsByRouteID(routeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch route stops"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"route": route,
		"stops": stops,
	})
}

// Helper function to format distance
func formatDistance(km float64) string {
	if km >= 1 {
		return fmt.Sprintf("%.1f km", km)
	}
	return fmt.Sprintf("%.0f m", km*1000)
}
