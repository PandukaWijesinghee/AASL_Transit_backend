package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// SearchService handles business logic for trip search
type SearchService struct {
	repo   *database.SearchRepository
	logger *logrus.Logger
}

// NewSearchService creates a new search service
func NewSearchService(repo *database.SearchRepository, logger *logrus.Logger) *SearchService {
	return &SearchService{
		repo:   repo,
		logger: logger,
	}
}

// SearchTrips searches for available trips between two locations
func (s *SearchService) SearchTrips(
	req *models.SearchRequest,
	userID *uuid.UUID,
	ipAddress string,
) (*models.SearchResponse, error) {
	startTime := time.Now()

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"from":    req.From,
		"to":      req.To,
		"user_id": userID,
	}).Info("Processing search request")

	// Initialize response
	response := &models.SearchResponse{
		Status: "success",
		SearchDetails: models.SearchDetails{
			FromStop: models.StopInfo{
				OriginalInput: req.From,
				Matched:       false,
			},
			ToStop: models.StopInfo{
				OriginalInput: req.To,
				Matched:       false,
			},
			SearchType: "exact",
		},
		Results: []models.TripResult{},
	}

	// Step 1: Find FROM stop
	fromStopInfo, fromStopID, err := s.repo.FindStopByName(req.From)
	if err != nil {
		s.logger.WithError(err).Error("Error finding from stop")
		return nil, fmt.Errorf("error searching for origin stop: %w", err)
	}

	response.SearchDetails.FromStop = *fromStopInfo

	if fromStopID == nil {
		response.Status = "partial"
		response.Message = fmt.Sprintf("Origin stop '%s' not found. Please check spelling or try a nearby location.", req.From)
		response.SearchDetails.SearchType = "failed"
		s.logSearch(req, response, userID, &ipAddress, time.Since(startTime))
		return response, nil
	}

	// Step 2: Find TO stop
	toStopInfo, toStopID, err := s.repo.FindStopByName(req.To)
	if err != nil {
		s.logger.WithError(err).Error("Error finding to stop")
		return nil, fmt.Errorf("error searching for destination stop: %w", err)
	}

	response.SearchDetails.ToStop = *toStopInfo

	if toStopID == nil {
		response.Status = "partial"
		response.Message = fmt.Sprintf("Destination stop '%s' not found. Please check spelling or try a nearby location.", req.To)
		response.SearchDetails.SearchType = "failed"
		s.logSearch(req, response, userID, &ipAddress, time.Since(startTime))
		return response, nil
	}

	// Step 3: Check if both stops are on the same route
	if fromStopID.String() == toStopID.String() {
		response.Status = "error"
		response.Message = "Origin and destination cannot be the same stop"
		response.SearchDetails.SearchType = "failed"
		s.logSearch(req, response, userID, &ipAddress, time.Since(startTime))
		return response, nil
	}

	// Step 4: Get search datetime (default to now if not provided)
	searchTime := req.GetSearchDateTime()

	// Step 5: Find available trips
	trips, err := s.repo.FindDirectTrips(*fromStopID, *toStopID, searchTime, req.Limit)
	if err != nil {
		s.logger.WithError(err).Error("Error finding trips")
		return nil, fmt.Errorf("error searching for trips: %w", err)
	}

	response.Results = trips

	// Step 6: Build appropriate message
	if len(trips) == 0 {
		response.Status = "success"
		response.Message = fmt.Sprintf(
			"No direct trips found from %s to %s. Try searching for a different date or nearby stops.",
			fromStopInfo.Name,
			toStopInfo.Name,
		)
	} else {
		response.Status = "success"
		response.Message = fmt.Sprintf(
			"Found %d trip(s) from %s to %s",
			len(trips),
			fromStopInfo.Name,
			toStopInfo.Name,
		)
	}

	// Step 7: Calculate search time
	responseTime := time.Since(startTime)
	response.SearchTimeMs = responseTime.Milliseconds()

	// Step 8: Log search for analytics
	s.logSearch(req, response, userID, &ipAddress, responseTime)

	s.logger.WithFields(logrus.Fields{
		"from":         req.From,
		"to":           req.To,
		"results":      len(trips),
		"response_ms":  response.SearchTimeMs,
	}).Info("Search completed successfully")

	return response, nil
}

// GetPopularRoutes returns popular routes for quick selection
func (s *SearchService) GetPopularRoutes(limit int) ([]models.PopularRoute, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	routes, err := s.repo.GetPopularRoutes(limit)
	if err != nil {
		s.logger.WithError(err).Error("Error getting popular routes")
		return nil, fmt.Errorf("error retrieving popular routes: %w", err)
	}

	// If no popular routes from analytics, return hardcoded popular routes
	if len(routes) == 0 {
		routes = s.getDefaultPopularRoutes()
	}

	return routes, nil
}

// GetStopAutocomplete returns stop suggestions for autocomplete
func (s *SearchService) GetStopAutocomplete(searchTerm string, limit int) ([]models.StopAutocomplete, error) {
	if searchTerm == "" || len(searchTerm) < 2 {
		return []models.StopAutocomplete{}, nil
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	suggestions, err := s.repo.GetStopAutocomplete(searchTerm, limit)
	if err != nil {
		s.logger.WithError(err).Error("Error getting autocomplete suggestions")
		return nil, fmt.Errorf("error retrieving suggestions: %w", err)
	}

	return suggestions, nil
}

// GetSearchAnalytics returns search analytics for admin dashboard
func (s *SearchService) GetSearchAnalytics(days int) (map[string]interface{}, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}

	analytics, err := s.repo.GetSearchAnalytics(days)
	if err != nil {
		s.logger.WithError(err).Error("Error getting search analytics")
		return nil, fmt.Errorf("error retrieving analytics: %w", err)
	}

	return analytics, nil
}

// logSearch logs the search request for analytics
func (s *SearchService) logSearch(
	req *models.SearchRequest,
	response *models.SearchResponse,
	userID *uuid.UUID,
	ipAddress *string,
	responseTime time.Duration,
) {
	log := &models.SearchLog{
		FromInput:      req.From,
		ToInput:        req.To,
		ResultsCount:   len(response.Results),
		ResponseTimeMs: responseTime.Milliseconds(),
		UserID:         userID,
		IPAddress:      ipAddress,
	}

	// Add stop IDs if matched
	if response.SearchDetails.FromStop.ID != nil {
		log.FromStopID = response.SearchDetails.FromStop.ID
	}
	if response.SearchDetails.ToStop.ID != nil {
		log.ToStopID = response.SearchDetails.ToStop.ID
	}

	// Log asynchronously to not block response
	go func() {
		if err := s.repo.LogSearch(log); err != nil {
			s.logger.WithError(err).Warn("Failed to log search")
		}
	}()
}

// getDefaultPopularRoutes returns hardcoded popular routes for Sri Lanka
func (s *SearchService) getDefaultPopularRoutes() []models.PopularRoute {
	return []models.PopularRoute{
		{FromStopName: "Colombo Fort", ToStopName: "Kandy", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Galle", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Anuradhapura", RouteCount: 0},
		{FromStopName: "Kandy", ToStopName: "Nuwara Eliya", RouteCount: 0},
		{FromStopName: "Galle", ToStopName: "Matara", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Jaffna", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Trincomalee", RouteCount: 0},
		{FromStopName: "Kandy", ToStopName: "Ella", RouteCount: 0},
		{FromStopName: "Negombo", ToStopName: "Colombo Fort", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Katunayake", RouteCount: 0},
	}
}
