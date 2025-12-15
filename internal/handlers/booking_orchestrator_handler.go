package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// BookingOrchestratorHandler handles booking intent and confirmation endpoints
type BookingOrchestratorHandler struct {
	orchestratorService *services.BookingOrchestratorService
	logger              *logrus.Logger
}

// NewBookingOrchestratorHandler creates a new BookingOrchestratorHandler
func NewBookingOrchestratorHandler(
	orchestratorService *services.BookingOrchestratorService,
	logger *logrus.Logger,
) *BookingOrchestratorHandler {
	return &BookingOrchestratorHandler{
		orchestratorService: orchestratorService,
		logger:              logger,
	}
}

// ============================================================================
// CREATE INTENT - POST /api/v1/booking/intent
// ============================================================================

// CreateIntent creates a new booking intent with TTL-based seat/lounge holding
// @Summary Create booking intent
// @Description Creates a booking intent, holds seats/lounges for TTL period
// @Tags Booking Orchestration
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body models.CreateBookingIntentRequest true "Booking intent request"
// @Success 201 {object} models.BookingIntentResponse
// @Failure 400 {object} map[string]interface{} "Validation error or seats unavailable"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 409 {object} models.PartialAvailabilityError "Partial availability"
// @Router /booking/intent [post]
func (h *BookingOrchestratorHandler) CreateIntent(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Parse request body
	var req models.CreateBookingIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Create intent
	response, err := h.orchestratorService.CreateIntent(userID, &req)
	if err != nil {
		// Check if it's a partial availability error
		if partialErr, ok := err.(*models.PartialAvailabilityError); ok {
			c.JSON(http.StatusConflict, gin.H{
				"error":       "partial_availability",
				"available":   partialErr.Available,
				"unavailable": partialErr.Unavailable,
				"message":     partialErr.Message,
			})
			return
		}

		h.logger.WithError(err).Error("Failed to create booking intent")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// ============================================================================
// INITIATE PAYMENT - POST /api/v1/booking/intent/:intent_id/initiate-payment
// ============================================================================

// InitiatePayment initiates payment for a booking intent
// @Summary Initiate payment for intent
// @Description Returns payment gateway URL and details
// @Tags Booking Orchestration
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param intent_id path string true "Intent ID"
// @Success 200 {object} models.InitiatePaymentResponse
// @Failure 400 {object} map[string]interface{} "Intent expired or invalid state"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Intent not found"
// @Router /booking/intent/{intent_id}/initiate-payment [post]
func (h *BookingOrchestratorHandler) InitiatePayment(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Parse intent ID from URL
	intentIDStr := c.Param("intent_id")
	intentID, err := uuid.Parse(intentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid intent_id"})
		return
	}

	// Initiate payment
	response, err := h.orchestratorService.InitiatePayment(intentID, userID)
	if err != nil {
		if err.Error() == "intent not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: intent belongs to another user" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// CONFIRM BOOKING - POST /api/v1/booking/confirm
// ============================================================================

// ConfirmBooking confirms a booking intent after payment
// @Summary Confirm booking after payment
// @Description Creates actual bookings from the intent
// @Tags Booking Orchestration
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body models.ConfirmBookingRequest true "Confirm booking request"
// @Success 200 {object} models.ConfirmBookingResponse
// @Failure 400 {object} map[string]interface{} "Intent expired or invalid state"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 402 {object} map[string]interface{} "Payment not verified"
// @Failure 404 {object} map[string]interface{} "Intent not found"
// @Failure 409 {object} map[string]interface{} "Seats no longer available"
// @Router /booking/confirm [post]
func (h *BookingOrchestratorHandler) ConfirmBooking(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Parse request body
	var req models.ConfirmBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Parse intent ID
	intentID, err := uuid.Parse(req.IntentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid intent_id"})
		return
	}

	// Confirm booking
	response, err := h.orchestratorService.ConfirmBooking(intentID, userID, req.PaymentReference)
	if err != nil {
		if err.Error() == "intent not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: intent belongs to another user" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "intent has expired, seats have been released" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "intent_expired",
				"message": err.Error(),
			})
			return
		}

		h.logger.WithError(err).Error("Failed to confirm booking")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// GET INTENT STATUS - GET /api/v1/booking/intent/:intent_id
// ============================================================================

// GetIntentStatus retrieves the current status of a booking intent
// @Summary Get booking intent status
// @Description Returns intent details including status, pricing, and bookings if confirmed
// @Tags Booking Orchestration
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param intent_id path string true "Intent ID"
// @Success 200 {object} models.GetIntentStatusResponse
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Intent not found"
// @Router /booking/intent/{intent_id} [get]
func (h *BookingOrchestratorHandler) GetIntentStatus(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Parse intent ID from URL
	intentIDStr := c.Param("intent_id")
	intentID, err := uuid.Parse(intentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid intent_id"})
		return
	}

	// Get status
	response, err := h.orchestratorService.GetIntentStatus(intentID, userID)
	if err != nil {
		if err.Error() == "intent not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// CANCEL INTENT - POST /api/v1/booking/intent/:intent_id/cancel
// ============================================================================

// CancelIntent cancels a booking intent and releases all holds
// @Summary Cancel booking intent
// @Description Cancels intent and releases all seat/lounge holds
// @Tags Booking Orchestration
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param intent_id path string true "Intent ID"
// @Success 200 {object} map[string]interface{} "Intent cancelled"
// @Failure 400 {object} map[string]interface{} "Cannot cancel confirmed intent"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Intent not found"
// @Router /booking/intent/{intent_id}/cancel [post]
func (h *BookingOrchestratorHandler) CancelIntent(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Parse intent ID from URL
	intentIDStr := c.Param("intent_id")
	intentID, err := uuid.Parse(intentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid intent_id"})
		return
	}

	// Cancel intent
	err = h.orchestratorService.CancelIntent(intentID, userID)
	if err != nil {
		if err.Error() == "intent not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Booking intent cancelled successfully",
		"intent_id": intentID,
	})
}

// ============================================================================
// PAYMENT WEBHOOK - POST /api/v1/payments/webhook
// ============================================================================

// PaymentWebhook handles payment gateway webhook callbacks
// @Summary Payment webhook callback
// @Description Called by payment gateway (PAYable) to notify of payment status
// @Tags Booking Orchestration
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Webhook payload from gateway"
// @Success 200 {object} map[string]interface{} "Webhook processed"
// @Failure 400 {object} map[string]interface{} "Invalid webhook"
// @Router /payments/webhook [post]
func (h *BookingOrchestratorHandler) PaymentWebhook(c *gin.Context) {
	// Parse webhook payload
	var payload struct {
		InvoiceID     string `json:"invoiceId" binding:"required"`
		Status        string `json:"status" binding:"required"`
		TransactionID string `json:"transactionId"`
		Amount        string `json:"amount"`
		Currency      string `json:"currency"`
		Signature     string `json:"signature"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook payload"})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"invoice_id":     payload.InvoiceID,
		"status":         payload.Status,
		"transaction_id": payload.TransactionID,
	}).Info("Payment webhook received")

	// TODO: Verify webhook signature from payment gateway
	// For now, we trust the webhook (in production, MUST verify signature)

	// Only process SUCCESS status
	if payload.Status != "SUCCESS" && payload.Status != "success" {
		h.logger.WithField("status", payload.Status).Info("Payment webhook - non-success status, ignoring")
		c.JSON(http.StatusOK, gin.H{"message": "acknowledged"})
		return
	}

	// Extract intent ID from invoice ID (format: INT-xxxxxxxx)
	// In production, would look up by payment_reference
	// For now, we just acknowledge
	c.JSON(http.StatusOK, gin.H{
		"message": "webhook processed",
		"status":  "acknowledged",
	})
}

// ============================================================================
// GET MY INTENTS - GET /api/v1/booking/intents
// ============================================================================

// GetMyIntents retrieves all booking intents for the current user
// @Summary Get my booking intents
// @Description Returns all intents for the authenticated user
// @Tags Booking Orchestration
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param limit query int false "Limit results (default 20)"
// @Param offset query int false "Offset for pagination"
// @Success 200 {array} models.BookingIntent
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Router /booking/intents [get]
func (h *BookingOrchestratorHandler) GetMyIntents(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Parse pagination
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil || limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
	}
	if o := c.Query("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	// Get user's intents from service
	intents, err := h.orchestratorService.GetIntentsByUser(userID, limit, offset)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user intents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get intents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"intents": intents,
		"limit":   limit,
		"offset":  offset,
	})
}
