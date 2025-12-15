package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// BookingOrchestratorHandler handles booking intent and confirmation endpoints
type BookingOrchestratorHandler struct {
	orchestratorService *services.BookingOrchestratorService
	payableService      *services.PAYableService
	logger              *logrus.Logger
}

// NewBookingOrchestratorHandler creates a new BookingOrchestratorHandler
func NewBookingOrchestratorHandler(
	orchestratorService *services.BookingOrchestratorService,
	payableService *services.PAYableService,
	logger *logrus.Logger,
) *BookingOrchestratorHandler {
	return &BookingOrchestratorHandler{
		orchestratorService: orchestratorService,
		payableService:      payableService,
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
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID := userCtx.UserID

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
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID := userCtx.UserID

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
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID := userCtx.UserID

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
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID := userCtx.UserID

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
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID := userCtx.UserID

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
		"message":   "Booking intent cancelled successfully",
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
	// Read raw body for verification
	bodyBytes, err := c.GetRawData()
	if err != nil {
		h.logger.WithError(err).Error("Failed to read webhook body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Verify and parse the webhook using PAYable service
	if h.payableService == nil {
		h.logger.Warn("PAYable service not configured - accepting webhook without verification")
	}

	// Parse the webhook payload
	payload, err := h.payableService.VerifyWebhook(bodyBytes)
	if err != nil {
		h.logger.WithError(err).Warn("Failed to verify webhook payload")
		// Still return 200 to acknowledge receipt (prevents retries)
		c.JSON(http.StatusOK, gin.H{"error": "invalid webhook payload", "acknowledged": true})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"uid":            payload.UID,
		"invoice_id":     payload.InvoiceID,
		"payment_status": payload.PaymentStatus,
		"amount":         payload.Amount,
		"transaction_id": payload.TransactionID,
	}).Info("PAYable webhook received")

	// Check if payment was successful
	if !h.payableService.IsPaymentSuccessful(payload) {
		h.logger.WithFields(logrus.Fields{
			"uid":            payload.UID,
			"payment_status": payload.PaymentStatus,
		}).Info("Payment not successful - acknowledging webhook")
		c.JSON(http.StatusOK, gin.H{
			"message": "webhook acknowledged",
			"status":  payload.PaymentStatus,
		})
		return
	}

	// Payment successful - confirm the booking
	// Look up intent by payment UID
	intent, err := h.orchestratorService.GetIntentByPaymentUID(payload.UID)
	if err != nil || intent == nil {
		h.logger.WithFields(logrus.Fields{
			"uid":        payload.UID,
			"invoice_id": payload.InvoiceID,
		}).Warn("Intent not found for webhook - may be duplicate or stale")
		c.JSON(http.StatusOK, gin.H{
			"message": "webhook acknowledged",
			"note":    "intent not found",
		})
		return
	}

	// Confirm the booking
	h.logger.WithFields(logrus.Fields{
		"intent_id":      intent.ID,
		"uid":            payload.UID,
		"transaction_id": payload.TransactionID,
	}).Info("Confirming booking from webhook")

	_, err = h.orchestratorService.ConfirmBooking(
		intent.ID,
		intent.UserID,
		&payload.TransactionID,
	)
	if err != nil {
		h.logger.WithError(err).Error("Failed to confirm booking from webhook")
		// Still acknowledge the webhook to prevent retries
		// The user can manually confirm or retry
		c.JSON(http.StatusOK, gin.H{
			"message": "webhook acknowledged",
			"error":   "confirmation failed",
		})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"intent_id":      intent.ID,
		"uid":            payload.UID,
		"transaction_id": payload.TransactionID,
	}).Info("Booking confirmed via webhook")

	c.JSON(http.StatusOK, gin.H{
		"message": "webhook processed successfully",
		"status":  "confirmed",
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
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID := userCtx.UserID

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
