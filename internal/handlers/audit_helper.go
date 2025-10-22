package handlers

import (
	"log"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/services"
	"time"
)

// logAuditError is a helper to log audit service errors without failing the request
func logAuditError(operation string, err error) {
	if err != nil {
		log.Printf("AUDIT ERROR [%s]: %v", operation, err)
	}
}

// Helper functions to log audit events with error handling

func (h *AuthHandler) safeLogOTPRequest(phone, ipAddress, userAgent string, success bool, reason string) {
	if err := h.auditService.LogOTPRequest(phone, ipAddress, userAgent, success, reason); err != nil {
		logAuditError("LogOTPRequest", err)
	}
}

func (h *AuthHandler) safeLogOTPVerification(userID *uuid.UUID, phone string, success bool, attempts int, ipAddress, userAgent, failureReason string) {
	if err := h.auditService.LogOTPVerification(userID, phone, success, attempts, ipAddress, userAgent, failureReason); err != nil {
		logAuditError("LogOTPVerification", err)
	}
}

func (h *AuthHandler) safeLogRateLimitViolation(phone, ipAddress, userAgent, limitType string, retryAfter time.Time) {
	if err := h.auditService.LogRateLimitViolation(phone, ipAddress, userAgent, limitType, retryAfter); err != nil {
		logAuditError("LogRateLimitViolation", err)
	}
}

func (h *AuthHandler) safeLogLogin(userID uuid.UUID, phone, ipAddress, userAgent, deviceID, deviceType string) {
	if err := h.auditService.LogLogin(userID, phone, ipAddress, userAgent, deviceID, deviceType); err != nil {
		logAuditError("LogLogin", err)
	}
}

func (h *AuthHandler) safeLogLogout(userID uuid.UUID, ipAddress, userAgent string, logoutAll bool) {
	if err := h.auditService.LogLogout(userID, ipAddress, userAgent, logoutAll); err != nil {
		logAuditError("LogLogout", err)
	}
}

func (h *AuthHandler) safeLogTokenRefresh(userID uuid.UUID, ipAddress, userAgent string, success bool) {
	if err := h.auditService.LogTokenRefresh(userID, ipAddress, userAgent, success); err != nil {
		logAuditError("LogTokenRefresh", err)
	}
}
