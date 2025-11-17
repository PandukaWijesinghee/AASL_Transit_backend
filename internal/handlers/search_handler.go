package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// SearchHandler handles HTTP requests for trip search
type SearchHandler struct {
	service *services.SearchService
	logger  *logrus.Logger
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(service *services.SearchService, logger *logrus.Logger) *SearchHandler {
	return &SearchHandler{
		service: service,
		logger:  logger,
	}
}

// SearchTrips handles POST /api/v1/search
// @Summary Search for available trips
// @Description Search for bus trips between two locations
// @Tags Search
// @Accept json
// @Produce json
// @Param search body models.SearchRequest true "Search parameters"
// @Success 200 {object} models.SearchResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/search [post]
func (h *SearchHandler) SearchTrips(c *gin.Context) {
	var req models.SearchRequest

	h.logger.Info("=== SEARCH REQUEST STARTED ===")

	// Log request headers for debugging
	h.logger.WithFields(logrus.Fields{
		"content_type":   c.GetHeader("Content-Type"),
		"content_length": c.GetHeader("Content-Length"),
		"user_agent":     c.GetHeader("User-Agent"),
		"authorization":  c.GetHeader("Authorization") != "",
	}).Info("Request headers received")

	// Parse request body (let Gin handle body reading internally)
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Warn("Invalid search request - JSON parsing failed")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
			"error":   err.Error(),
		})
		return
	}

	// Log search parameters
	h.logger.WithFields(logrus.Fields{
		"from":     req.From,
		"to":       req.To,
		"datetime": req.DateTime,
		"limit":    req.Limit,
	}).Info("Search parameters parsed successfully")

	// Get user ID from context (if authenticated)
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &uid
			h.logger.WithField("user_id", uid).Info("Authenticated user search")
		}
	}

	// Get client IP
	ipAddress := c.ClientIP()
	h.logger.WithField("ip", ipAddress).Info("Client IP captured")

	// Perform search
	h.logger.Info("Calling search service...")
	response, err := h.service.SearchTrips(&req, userID, ipAddress)
	if err != nil {
		// Check if it's a validation error
		if _, ok := err.(*models.ValidationError); ok {
			h.logger.WithError(err).Warn("Validation error in search request")
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		// Internal server error
		h.logger.WithError(err).Error("SEARCH FAILED - Internal error during search execution")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to search for trips. Please try again later.",
			"error":   err.Error(),
		})
		return
	}

	// Log successful response
	h.logger.WithFields(logrus.Fields{
		"results_count": len(response.Results),
		"search_time_ms": response.SearchTimeMs,
		"status": response.Status,
	}).Info("Search completed successfully")

	h.logger.Info("=== SEARCH REQUEST COMPLETED ===")

	// Return successful response
	c.JSON(http.StatusOK, response)
}

// GetPopularRoutes handles GET /api/v1/search/popular
// @Summary Get popular routes
// @Description Get frequently searched routes for quick selection
// @Tags Search
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of routes" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/search/popular [get]
func (h *SearchHandler) GetPopularRoutes(c *gin.Context) {
	// Get limit from query params (default: 10)
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	routes, err := h.service.GetPopularRoutes(limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get popular routes")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve popular routes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Popular routes retrieved successfully",
		"routes":  routes,
		"count":   len(routes),
	})
}

// GetStopAutocomplete handles GET /api/v1/search/autocomplete
// @Summary Get stop autocomplete suggestions
// @Description Get bus stop suggestions for autocomplete
// @Tags Search
// @Accept json
// @Produce json
// @Param q query string true "Search term"
// @Param limit query int false "Maximum number of suggestions" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/search/autocomplete [get]
func (h *SearchHandler) GetStopAutocomplete(c *gin.Context) {
	// Get search term from query params
	searchTerm := c.Query("q")
	if searchTerm == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Search term 'q' is required",
		})
		return
	}

	// Get limit from query params (default: 10)
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	suggestions, err := h.service.GetStopAutocomplete(searchTerm, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get autocomplete suggestions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve suggestions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"suggestions": suggestions,
		"count":       len(suggestions),
	})
}

// GetSearchAnalytics handles GET /api/v1/admin/search/analytics
// @Summary Get search analytics
// @Description Get search analytics for admin dashboard (requires admin auth)
// @Tags Admin, Search
// @Accept json
// @Produce json
// @Param days query int false "Number of days to analyze" default(7)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security Bearer
// @Router /api/v1/admin/search/analytics [get]
func (h *SearchHandler) GetSearchAnalytics(c *gin.Context) {
	// Get days from query params (default: 7)
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 {
			days = parsedDays
		}
	}

	analytics, err := h.service.GetSearchAnalytics(days)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get search analytics")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve analytics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"analytics": analytics,
		"period":    days,
	})
}

// HealthCheck handles GET /api/v1/search/health
// @Summary Search service health check
// @Description Check if search service is healthy and database is accessible
// @Tags Search
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{} "Service unavailable"
// @Router /api/v1/search/health [get]
func (h *SearchHandler) HealthCheck(c *gin.Context) {
	// In a production app, you might want to ping the database here
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "search",
		"message": "Search service is operational",
	})
}
