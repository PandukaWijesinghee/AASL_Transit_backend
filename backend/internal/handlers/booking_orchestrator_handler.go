package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// BookingOrchestratorHandler handles booking intent and confirmation endpoints
type BookingOrchestratorHandler struct {
	orchestratorService *services.BookingOrchestratorService
	payableService      *services.PAYableService
	paymentAuditRepo    *database.PaymentAuditRepository
	logger              *logrus.Logger
}

// NewBookingOrchestratorHandler creates a new BookingOrchestratorHandler
func NewBookingOrchestratorHandler(
	orchestratorService *services.BookingOrchestratorService,
	payableService *services.PAYableService,
	paymentAuditRepo *database.PaymentAuditRepository,
	logger *logrus.Logger,
) *BookingOrchestratorHandler {
	return &BookingOrchestratorHandler{
		orchestratorService: orchestratorService,
		payableService:      payableService,
		paymentAuditRepo:    paymentAuditRepo,
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
// ADD LOUNGE TO INTENT - PATCH /api/v1/booking/intent/{intent_id}/add-lounge
// ============================================================================

// AddLoungeToIntentRequest represents the request to add lounges to an existing intent
type AddLoungeToIntentRequest struct {
	PreTripLounge  *models.LoungeIntentPayload `json:"pre_trip_lounge,omitempty"`
	PostTripLounge *models.LoungeIntentPayload `json:"post_trip_lounge,omitempty"`
}

// AddLoungeToIntent adds pre-trip and/or post-trip lounge to an existing bus intent
// This keeps the seat hold active and extends the expiration time
// @Summary Add lounge to existing intent
// @Description Adds lounge(s) to an existing bus-only intent, extending the hold timer
// @Tags Booking Orchestration
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param intent_id path string true "Intent ID"
// @Param request body AddLoungeToIntentRequest true "Lounge(s) to add"
// @Success 200 {object} models.BookingIntentResponse "Updated intent with lounges"
// @Failure 400 {object} map[string]interface{} "Invalid request or intent status"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Intent not found"
// @Router /booking/intent/{intent_id}/add-lounge [patch]
func (h *BookingOrchestratorHandler) AddLoungeToIntent(c *gin.Context) {
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

	// Parse request body
	var req AddLoungeToIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Validate at least one lounge is provided
	if req.PreTripLounge == nil && req.PostTripLounge == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one lounge (pre_trip_lounge or post_trip_lounge) must be provided"})
		return
	}

	// Add lounges to intent
	response, err := h.orchestratorService.AddLoungeToIntent(intentID, userID, req.PreTripLounge, req.PostTripLounge)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "intent not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
			return
		}
		if errMsg == "unauthorized" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": errMsg})
			return
		}
		if strings.Contains(errMsg, "has expired") || strings.Contains(errMsg, "status") {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// PAYMENT WEBHOOK - POST /api/v1/payments/webhook
// Industry-standard implementation with:
// - Audit logging of ALL events
// - Idempotency (duplicate detection)
// - Amount verification
// - Proper error handling without silent failures
// ============================================================================

// PaymentWebhook handles payment gateway webhook callbacks
// @Summary Payment webhook callback
// @Description Called by payment gateway (PAYable) to notify of payment status.
//
//	This endpoint verifies payment with the gateway, validates amounts,
//	and confirms bookings with full audit trail.
//
// @Tags Booking Orchestration
// @Accept json
// @Produce json
// @Param uid query string true "Payment UID from PAYable"
// @Param statusIndicator query string true "Status indicator from PAYable"
// @Success 200 {object} map[string]interface{} "Webhook processed"
// @Failure 400 {object} map[string]interface{} "Invalid webhook"
// @Router /payments/webhook [post]
func (h *BookingOrchestratorHandler) PaymentWebhook(c *gin.Context) {
	ctx := context.Background()
	startTime := time.Now()

	// Extract request metadata
	uid := c.Query("uid")
	statusIndicator := c.Query("statusIndicator")
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	correlationID := c.GetHeader("X-Correlation-ID")
	if correlationID == "" {
		correlationID = uuid.New().String()
	}

	// Create audit entry for webhook receipt
	webhookAudit := models.NewPaymentAudit(models.PaymentEventWebhookReceived, models.PaymentSourcePayableWebhook)
	webhookAudit.SetPaymentUID(uid)
	webhookAudit.SetMetadata(clientIP, userAgent, correlationID)
	webhookAudit.SetIdempotencyKey(fmt.Sprintf("%s-webhook", uid))

	h.logger.WithFields(logrus.Fields{
		"uid":              uid,
		"status_indicator": statusIndicator,
		"correlation_id":   correlationID,
	}).Info("PAYable webhook received")

	// Validate query params
	if uid == "" || statusIndicator == "" {
		h.logger.Warn("Webhook missing uid or statusIndicator query params")
		webhookAudit.SetError("missing uid or statusIndicator", nil)
		h.logAudit(ctx, webhookAudit, startTime)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "missing uid or statusIndicator",
			"correlation_id": correlationID,
		})
		return
	}

	// Check for duplicate webhook (idempotency)
	if h.paymentAuditRepo != nil {
		isDuplicate, err := h.paymentAuditRepo.CheckDuplicate(ctx, uid, models.PaymentEventSuccess, fmt.Sprintf("%s-success", uid))
		if err != nil {
			h.logger.WithError(err).Warn("Failed to check for duplicate webhook")
		} else if isDuplicate {
			h.logger.WithFields(logrus.Fields{
				"uid":            uid,
				"correlation_id": correlationID,
			}).Info("Duplicate webhook detected - already processed successfully")
			webhookAudit.MarkAsDuplicate()
			h.logAudit(ctx, webhookAudit, startTime)
			c.JSON(http.StatusOK, gin.H{
				"message":        "webhook already processed",
				"duplicate":      true,
				"correlation_id": correlationID,
			})
			return
		}
	}

	// Log the webhook receipt
	h.logAudit(ctx, webhookAudit, startTime)

	// Verify PAYable service is configured
	if h.payableService == nil {
		h.logger.Error("PAYable service not configured")
		errorAudit := models.NewPaymentAudit(models.PaymentEventError, models.PaymentSourceBackend)
		errorAudit.SetPaymentUID(uid)
		errorAudit.SetError("payment service not configured", nil)
		h.logAudit(ctx, errorAudit, startTime)
		c.JSON(http.StatusOK, gin.H{
			"error":          "payment service not configured",
			"acknowledged":   true,
			"correlation_id": correlationID,
		})
		return
	}

	// Call PAYable CheckStatus API to get actual payment result
	statusCheckAudit := models.NewPaymentAudit(models.PaymentEventStatusCheckRequest, models.PaymentSourceBackend)
	statusCheckAudit.SetPaymentUID(uid)
	statusCheckAudit.SetRequestPayload(map[string]interface{}{
		"uid":             uid,
		"statusIndicator": statusIndicator,
	})
	h.logAudit(ctx, statusCheckAudit, startTime)

	// Try status check with retry for sandbox (sometimes returns empty status)
	var statusResp *services.PAYableStatusResponse
	var rawBody string
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		statusResp, rawBody, err = h.payableService.CheckStatusWithRawResponse(uid, statusIndicator)
		if err != nil {
			break // Fatal error, don't retry
		}

		// If we got a valid status, we're done
		if statusResp != nil && statusResp.GetPaymentStatus() != "" {
			break
		}

		// Empty status - sandbox quirk, retry after short delay
		if attempt < maxRetries {
			h.logger.WithFields(logrus.Fields{
				"uid":            uid,
				"attempt":        attempt,
				"max_retries":    maxRetries,
				"correlation_id": correlationID,
			}).Warn("PAYable returned empty payment_status - retrying after delay")
			time.Sleep(2 * time.Second)
		}
	}

	// Log the status check response (even if it fails)
	statusRespAudit := models.NewPaymentAudit(models.PaymentEventStatusCheckResponse, models.PaymentSourcePayableAPI)
	statusRespAudit.SetPaymentUID(uid)
	if rawBody != "" {
		statusRespAudit.SetRawBody(rawBody)
	}

	if err != nil {
		h.logger.WithError(err).Error("Failed to check payment status from PAYable")
		statusRespAudit.SetError(err.Error(), nil)
		h.logAudit(ctx, statusRespAudit, startTime)
		c.JSON(http.StatusOK, gin.H{
			"error":          "failed to verify payment status",
			"acknowledged":   true,
			"correlation_id": correlationID,
		})
		return
	}

	// Parse response into audit
	if statusResp != nil {
		statusRespAudit.SetPaymentStatus(statusResp.GetPaymentStatus())
		statusRespAudit.SetHTTPDetails("POST", "", statusResp.Status)
		txnID := statusResp.GetTransactionID()
		if txnID != "" {
			statusRespAudit.GatewayTransactionID = &txnID
		}
	}
	h.logAudit(ctx, statusRespAudit, startTime)

	h.logger.WithFields(logrus.Fields{
		"uid":            uid,
		"status":         statusResp.Status,
		"payment_status": statusResp.GetPaymentStatus(),
		"invoice_id":     statusResp.GetInvoiceID(),
		"amount":         statusResp.GetAmount(),
		"transaction_id": statusResp.GetTransactionID(),
		"correlation_id": correlationID,
	}).Info("PAYable status check response")

	// FIRST: Look up intent by payment UID to check if already confirmed
	intent, err := h.orchestratorService.GetIntentByPaymentUID(uid)

	// Check if booking was already confirmed (by Flutter via return URL)
	// This handles the race condition where Flutter confirms before webhook completes
	if intent != nil && intent.Status == models.IntentStatusConfirmed {
		h.logger.WithFields(logrus.Fields{
			"uid":            uid,
			"intent_id":      intent.ID,
			"intent_status":  intent.Status,
			"correlation_id": correlationID,
		}).Info("Booking already confirmed by client - acknowledging webhook")

		alreadyConfirmedAudit := models.NewPaymentAudit(models.PaymentEventSuccess, models.PaymentSourcePayableWebhook)
		alreadyConfirmedAudit.SetPaymentUID(uid)
		alreadyConfirmedAudit.SetIntent(intent.ID)
		alreadyConfirmedAudit.SetPaymentStatus("ALREADY_CONFIRMED")
		alreadyConfirmedAudit.SetIdempotencyKey(fmt.Sprintf("%s-already-confirmed", uid))
		h.logAudit(ctx, alreadyConfirmedAudit, startTime)

		c.JSON(http.StatusOK, gin.H{
			"message":        "webhook acknowledged",
			"note":           "booking already confirmed by client",
			"intent_id":      intent.ID,
			"correlation_id": correlationID,
		})
		return
	}

	// Check if payment was successful from PAYable API
	paymentStatus := strings.ToUpper(statusResp.GetPaymentStatus())
	if paymentStatus != "SUCCESS" {
		h.logger.WithFields(logrus.Fields{
			"uid":            uid,
			"payment_status": statusResp.GetPaymentStatus(),
			"correlation_id": correlationID,
		}).Info("Payment not successful - acknowledging webhook")

		// Log the failed/pending payment
		var eventType models.PaymentEventType
		switch paymentStatus {
		case "FAILED":
			eventType = models.PaymentEventFailed
		case "CANCELLED":
			eventType = models.PaymentEventCancelled
		default:
			// Still pending or unknown
			eventType = models.PaymentEventError
		}
		failAudit := models.NewPaymentAudit(eventType, models.PaymentSourcePayableAPI)
		failAudit.SetPaymentUID(uid)
		failAudit.SetPaymentStatus(statusResp.GetPaymentStatus())
		h.logAudit(ctx, failAudit, startTime)

		c.JSON(http.StatusOK, gin.H{
			"message":        "webhook acknowledged",
			"status":         statusResp.GetPaymentStatus(),
			"correlation_id": correlationID,
		})
		return
	}

	// Payment successful from PAYable - verify intent exists
	if err != nil || intent == nil {
		h.logger.WithFields(logrus.Fields{
			"uid":            uid,
			"invoice_id":     statusResp.GetInvoiceID(),
			"correlation_id": correlationID,
		}).Warn("Intent not found for webhook - may be duplicate or already processed")

		// Log this as a potential issue
		notFoundAudit := models.NewPaymentAudit(models.PaymentEventError, models.PaymentSourceBackend)
		notFoundAudit.SetPaymentUID(uid)
		notFoundAudit.SetError("intent not found - may be duplicate or already processed", nil)
		h.logAudit(ctx, notFoundAudit, startTime)

		c.JSON(http.StatusOK, gin.H{
			"message":        "webhook acknowledged",
			"note":           "intent not found or already processed",
			"correlation_id": correlationID,
		})
		return
	}

	// CRITICAL: Verify amount matches what we expect
	expectedAmount := intent.TotalAmount
	var receivedAmount float64
	receivedAmountStr := statusResp.GetAmount()
	if receivedAmountStr != "" {
		receivedAmount, _ = strconv.ParseFloat(receivedAmountStr, 64)
	}

	// Create success audit BEFORE confirming
	successAudit := models.NewPaymentAudit(models.PaymentEventSuccess, models.PaymentSourcePayableAPI)
	successAudit.SetPaymentUID(uid)
	successAudit.SetIntent(intent.ID)
	successAudit.SetPaymentReference(statusResp.GetInvoiceID())
	successAudit.SetPaymentStatus(statusResp.GetPaymentStatus())
	successAudit.SetIdempotencyKey(fmt.Sprintf("%s-success", uid))
	txnIDForSuccess := statusResp.GetTransactionID()
	if txnIDForSuccess != "" {
		successAudit.GatewayTransactionID = &txnIDForSuccess
	}

	// Verify amounts match
	amountsMatch := successAudit.SetAmounts(expectedAmount, receivedAmount, intent.Currency)
	if !amountsMatch {
		// CRITICAL: Amount mismatch - DO NOT confirm booking
		h.logger.WithFields(logrus.Fields{
			"uid":             uid,
			"expected_amount": expectedAmount,
			"received_amount": receivedAmount,
			"intent_id":       intent.ID,
			"correlation_id":  correlationID,
		}).Error("CRITICAL: Amount mismatch in payment - BLOCKING confirmation")

		successAudit.EventType = models.PaymentEventError
		successAudit.SetError(
			fmt.Sprintf("amount mismatch: expected %.2f, received %.2f", expectedAmount, receivedAmount),
			nil,
		)
		h.logAudit(ctx, successAudit, startTime)

		c.JSON(http.StatusOK, gin.H{
			"error":           "amount verification failed",
			"acknowledged":    true,
			"requires_review": true,
			"correlation_id":  correlationID,
		})
		return
	}

	h.logAudit(ctx, successAudit, startTime)

	// Confirm the booking
	h.logger.WithFields(logrus.Fields{
		"intent_id":      intent.ID,
		"uid":            uid,
		"amount":         receivedAmount,
		"transaction_id": statusResp.TransactionID,
		"correlation_id": correlationID,
	}).Info("Confirming booking from webhook - amount verified")

	bookingResult, err := h.orchestratorService.ConfirmBooking(
		intent.ID,
		intent.UserID,
		&statusResp.TransactionID,
	)

	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"intent_id":      intent.ID,
			"uid":            uid,
			"correlation_id": correlationID,
		}).Error("CRITICAL: Failed to confirm booking from webhook - payment received but booking failed")

		// Log the confirmation failure - THIS NEEDS MANUAL INTERVENTION
		failAudit := models.NewPaymentAudit(models.PaymentEventBookingConfirmFailed, models.PaymentSourceBackend)
		failAudit.SetPaymentUID(uid)
		failAudit.SetIntent(intent.ID)
		failAudit.SetError(err.Error(), nil)
		failAudit.SetAmounts(expectedAmount, receivedAmount, intent.Currency)
		h.logAudit(ctx, failAudit, startTime)

		c.JSON(http.StatusOK, gin.H{
			"message":         "webhook acknowledged",
			"error":           "booking confirmation failed",
			"requires_refund": true,
			"correlation_id":  correlationID,
		})
		return
	}

	// Log successful confirmation
	confirmAudit := models.NewPaymentAudit(models.PaymentEventBookingConfirmed, models.PaymentSourceBackend)
	confirmAudit.SetPaymentUID(uid)
	confirmAudit.SetIntent(intent.ID)
	confirmAudit.SetPaymentStatus("confirmed")
	confirmAudit.SetAmounts(expectedAmount, receivedAmount, intent.Currency)
	h.logAudit(ctx, confirmAudit, startTime)

	h.logger.WithFields(logrus.Fields{
		"intent_id":      intent.ID,
		"uid":            uid,
		"transaction_id": statusResp.TransactionID,
		"correlation_id": correlationID,
	}).Info("Booking confirmed via webhook successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":           "webhook processed successfully",
		"status":            "confirmed",
		"booking_reference": bookingResult.MasterReference,
		"correlation_id":    correlationID,
	})
}

// logAudit is a helper to log audit entries without blocking
func (h *BookingOrchestratorHandler) logAudit(ctx context.Context, audit *models.PaymentAudit, startTime time.Time) {
	if h.paymentAuditRepo == nil {
		h.logger.Warn("Payment audit repository not configured - audit NOT logged")
		return
	}

	audit.SetProcessingTime(startTime)
	if err := h.paymentAuditRepo.Log(ctx, audit); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"event_type":  audit.EventType,
			"payment_uid": audit.PaymentUID,
		}).Error("CRITICAL: Failed to log payment audit")
	}
}

// ============================================================================
// PAYMENT RETURN - GET /api/v1/payments/return
// ============================================================================

// PaymentReturn handles the return URL redirect from PAYable after payment
// @Summary Payment return handler
// @Description Called by PAYable to redirect user back after payment completion.
//
//	Returns a simple HTML page that tells the WebView payment is complete.
//
// @Tags Booking Orchestration
// @Param uid query string true "Payment UID from PAYable"
// @Param statusIndicator query string true "Status indicator from PAYable"
// @Success 200 {string} string "HTML page indicating payment complete"
// @Router /payments/return [get]
func (h *BookingOrchestratorHandler) PaymentReturn(c *gin.Context) {
	uid := c.Query("uid")
	statusIndicator := c.Query("statusIndicator")

	h.logger.WithFields(logrus.Fields{
		"uid":              uid,
		"status_indicator": statusIndicator,
	}).Info("Payment return page accessed")

	// Return a simple HTML page that the WebView can detect
	// The Flutter app should intercept this URL before it loads
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Payment Complete</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: #f5f5f5; }
        .container { background: white; padding: 40px; border-radius: 10px; max-width: 400px; margin: 0 auto; }
        .success { color: #4CAF50; font-size: 48px; }
        h1 { color: #333; }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="success">✓</div>
        <h1>Payment Complete</h1>
        <p>Your payment has been processed successfully.</p>
        <p>You can close this window and return to the app.</p>
    </div>
</body>
</html>`

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
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
