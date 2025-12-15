package services

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/config"
)

// PAYableEnvironmentURLs maps environment names to their IPG endpoint URLs
var PAYableEnvironmentURLs = map[string]string{
	"dev":        "https://payable-ipg-dev.web.app/ipg/dev",
	"sandbox":    "https://sandboxipgpayment.payable.lk/ipg/sandbox",
	"production": "https://ipgpayment.payable.lk/ipg/pro",
}

// PAYableService handles payment gateway integration with PAYable IPG
type PAYableService struct {
	config *config.PaymentConfig
	logger *logrus.Logger
	client *http.Client
}

// PAYablePaymentRequest represents the request sent to PAYable IPG
type PAYablePaymentRequest struct {
	// Merchant credentials
	MerchantKey   string `json:"merchantKey"`
	MerchantToken string `json:"merchantToken"`

	// URLs
	LogoURL    string `json:"logoUrl,omitempty"`
	ReturnURL  string `json:"returnUrl"`
	WebhookURL string `json:"webhookUrl,omitempty"`

	// Payment details
	PaymentType  int    `json:"paymentType"` // 1 = one-time, 2 = recurring
	InvoiceID    string `json:"invoiceId"`
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`

	// Customer details
	CustomerFirstName   string `json:"customerFirstName,omitempty"`
	CustomerLastName    string `json:"customerLastName,omitempty"`
	CustomerEmail       string `json:"customerEmail,omitempty"`
	CustomerMobilePhone string `json:"customerMobilePhone,omitempty"`

	// Billing address
	BillingAddressStreet      string `json:"billingAddressStreet,omitempty"`
	BillingAddressCity        string `json:"billingAddressCity,omitempty"`
	BillingAddressCountry     string `json:"billingAddressCountry,omitempty"`
	BillingAddressPostcodeZip string `json:"billingAddressPostcodeZip,omitempty"`

	// Order details
	OrderDescription string `json:"orderDescription,omitempty"`

	// Security
	CheckValue string `json:"checkValue"`

	// Integration info
	IsMobilePayment    int    `json:"isMobilePayment"`
	IntegrationType    string `json:"integrationType"`
	IntegrationVersion string `json:"integrationVersion"`
}

// PAYablePaymentResponse represents the response from PAYable IPG
type PAYablePaymentResponse struct {
	Status          string `json:"status"`            // "success" or "error"
	UID             string `json:"uid"`               // Unique transaction ID
	StatusIndicator string `json:"statusIndicator"`   // Token for status checks
	PaymentPage     string `json:"paymentPage"`       // URL to redirect user for payment
	Message         string `json:"message,omitempty"` // Error message if status is error
}

// PAYableStatusRequest represents the request to check payment status
type PAYableStatusRequest struct {
	UID             string `json:"uid"`
	StatusIndicator string `json:"statusIndicator"`
}

// PAYableStatusResponse represents the response from status check
type PAYableStatusResponse struct {
	Status        string `json:"status"`
	PaymentStatus string `json:"paymentStatus"` // "pending", "success", "failed", "cancelled"
	Amount        string `json:"amount"`
	InvoiceID     string `json:"invoiceId"`
	TransactionID string `json:"transactionId,omitempty"`
	Message       string `json:"message,omitempty"`
}

// PAYableWebhookPayload represents the webhook payload from PAYable
type PAYableWebhookPayload struct {
	Status          string `json:"status"`
	UID             string `json:"uid"`
	InvoiceID       string `json:"invoiceId"`
	Amount          string `json:"amount"`
	CurrencyCode    string `json:"currencyCode"`
	PaymentStatus   string `json:"paymentStatus"` // "SUCCESS", "FAILED", "CANCELLED"
	TransactionID   string `json:"transactionId,omitempty"`
	PaymentMethod   string `json:"paymentMethod,omitempty"`
	CardType        string `json:"cardType,omitempty"`
	CardLastFour    string `json:"cardLastFour,omitempty"`
	StatusIndicator string `json:"statusIndicator"`
}

// NewPAYableService creates a new PAYable payment service
func NewPAYableService(cfg *config.PaymentConfig, logger *logrus.Logger) *PAYableService {
	return &PAYableService{
		config: cfg,
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateCheckValue creates the SHA-512 checkValue for PAYable authentication
// Step 1: hash1 = SHA512(merchantToken) uppercase hex
// Step 2: hash2 = SHA512("merchantKey|invoiceId|amount|currencyCode|hash1") uppercase hex
func (s *PAYableService) GenerateCheckValue(invoiceID, amount, currencyCode string) string {
	// Step 1: SHA512 of merchant token
	hash1 := sha512.Sum512([]byte(s.config.MerchantToken))
	hash1Hex := strings.ToUpper(hex.EncodeToString(hash1[:]))

	// Step 2: SHA512 of concatenated string
	data := fmt.Sprintf("%s|%s|%s|%s|%s",
		s.config.MerchantKey,
		invoiceID,
		amount,
		currencyCode,
		hash1Hex,
	)
	hash2 := sha512.Sum512([]byte(data))
	return strings.ToUpper(hex.EncodeToString(hash2[:]))
}

// InitiatePaymentParams contains all parameters needed to initiate a payment
type InitiatePaymentParams struct {
	InvoiceID        string
	Amount           string
	CurrencyCode     string
	CustomerName     string // Will be split into first/last name
	CustomerPhone    string
	CustomerEmail    string
	OrderDescription string
}

// InitiatePayment creates a payment request and returns the payment page URL
func (s *PAYableService) InitiatePayment(params *InitiatePaymentParams) (*PAYablePaymentResponse, error) {
	// Validate config
	if s.config.MerchantKey == "" || s.config.MerchantToken == "" {
		return nil, fmt.Errorf("payment gateway not configured: missing merchant credentials")
	}

	// Generate checkValue
	checkValue := s.GenerateCheckValue(params.InvoiceID, params.Amount, params.CurrencyCode)

	// Split customer name
	firstName, lastName := s.splitName(params.CustomerName)

	// Build request
	request := &PAYablePaymentRequest{
		MerchantKey:           s.config.MerchantKey,
		MerchantToken:         s.config.MerchantToken,
		LogoURL:               s.config.LogoURL,
		ReturnURL:             s.config.ReturnURL,
		WebhookURL:            s.config.WebhookURL,
		PaymentType:           1, // One-time payment
		InvoiceID:             params.InvoiceID,
		Amount:                params.Amount,
		CurrencyCode:          params.CurrencyCode,
		CustomerFirstName:     firstName,
		CustomerLastName:      lastName,
		CustomerEmail:         params.CustomerEmail,
		CustomerMobilePhone:   params.CustomerPhone,
		BillingAddressCountry: "LK", // Sri Lanka
		OrderDescription:      params.OrderDescription,
		CheckValue:            checkValue,
		IsMobilePayment:       1,
		IntegrationType:       "Smart Transit Backend",
		IntegrationVersion:    "1.0.0",
	}

	s.logger.WithFields(logrus.Fields{
		"invoice_id": params.InvoiceID,
		"amount":     params.Amount,
		"currency":   params.CurrencyCode,
	}).Info("Initiating PAYable payment")

	// Get endpoint URL
	endpointURL, ok := PAYableEnvironmentURLs[s.config.Environment]
	if !ok {
		endpointURL = PAYableEnvironmentURLs["sandbox"] // Default to sandbox
	}

	// Make HTTP request
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := s.client.Post(endpointURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		s.logger.WithError(err).Error("Failed to call PAYable endpoint")
		return nil, fmt.Errorf("failed to call payment gateway: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"status_code": resp.StatusCode,
		"response":    string(body),
	}).Debug("PAYable response received")

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("payment gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var paymentResp PAYablePaymentResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if paymentResp.Status != "success" {
		return nil, fmt.Errorf("payment initiation failed: %s", paymentResp.Message)
	}

	s.logger.WithFields(logrus.Fields{
		"uid":          paymentResp.UID,
		"payment_page": paymentResp.PaymentPage,
	}).Info("PAYable payment initiated successfully")

	return &paymentResp, nil
}

// CheckStatus queries the current status of a payment
func (s *PAYableService) CheckStatus(uid, statusIndicator string) (*PAYableStatusResponse, error) {
	request := &PAYableStatusRequest{
		UID:             uid,
		StatusIndicator: statusIndicator,
	}

	// Status check endpoint
	statusURL := "https://endpoint.payable.lk/check-status"

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := s.client.Post(statusURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to check status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var statusResp PAYableStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &statusResp, nil
}

// VerifyWebhook validates and parses a webhook payload from PAYable
// Returns the parsed payload if valid, error otherwise
func (s *PAYableService) VerifyWebhook(body []byte) (*PAYableWebhookPayload, error) {
	var payload PAYableWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid webhook payload: %w", err)
	}

	// Basic validation
	if payload.UID == "" || payload.InvoiceID == "" {
		return nil, fmt.Errorf("webhook missing required fields")
	}

	// Additional validation could include:
	// 1. Verify the statusIndicator matches what we stored
	// 2. Verify the amount matches our records
	// 3. Check that the payment hasn't already been processed

	s.logger.WithFields(logrus.Fields{
		"uid":            payload.UID,
		"invoice_id":     payload.InvoiceID,
		"payment_status": payload.PaymentStatus,
		"amount":         payload.Amount,
	}).Info("Webhook payload verified")

	return &payload, nil
}

// IsPaymentSuccessful checks if a webhook indicates successful payment
func (s *PAYableService) IsPaymentSuccessful(payload *PAYableWebhookPayload) bool {
	return strings.ToUpper(payload.PaymentStatus) == "SUCCESS"
}

// splitName splits a full name into first and last name
func (s *PAYableService) splitName(fullName string) (firstName, lastName string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "Customer", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// IsConfigured returns true if payment gateway is properly configured
func (s *PAYableService) IsConfigured() bool {
	return s.config.MerchantKey != "" && s.config.MerchantToken != ""
}

// GetEnvironment returns the current payment environment
func (s *PAYableService) GetEnvironment() string {
	return s.config.Environment
}
