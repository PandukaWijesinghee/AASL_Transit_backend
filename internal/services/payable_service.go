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
// NOTE: merchantToken is NOT sent - PAYable rejects it. Only used for checkValue calculation.
type PAYablePaymentRequest struct {
	// Merchant credentials (only merchantKey is sent, token used for checkValue)
	MerchantKey string `json:"merchantKey"`

	// URLs
	LogoURL         string `json:"logoUrl,omitempty"`
	ReturnURL       string `json:"returnUrl"`
	WebhookURL      string `json:"webhookUrl,omitempty"`
	StatusReturnURL string `json:"statusReturnUrl,omitempty"`

	// Payment details
	PaymentType  int    `json:"paymentType"` // 1 = one-time, 2 = recurring
	InvoiceID    string `json:"invoiceId"`
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`

	// Order details
	OrderDescription string `json:"orderDescription,omitempty"`

	// Customer details (REQUIRED)
	CustomerFirstName   string `json:"customerFirstName"`
	CustomerLastName    string `json:"customerLastName"`
	CustomerEmail       string `json:"customerEmail"`
	CustomerMobilePhone string `json:"customerMobilePhone"`

	// Billing address (REQUIRED)
	BillingAddressStreet      string `json:"billingAddressStreet"`
	BillingAddressCity        string `json:"billingAddressCity"`
	BillingAddressCountry     string `json:"billingAddressCountry"`
	BillingAddressPostcodeZip string `json:"billingAddressPostcodeZip"`

	// Shipping address (same as billing for digital services)
	ShippingAddressStreet      string `json:"shippingAddressStreet,omitempty"`
	ShippingAddressCity        string `json:"shippingAddressCity,omitempty"`
	ShippingAddressCountry     string `json:"shippingAddressCountry,omitempty"`
	ShippingAddressPostcodeZip string `json:"shippingAddressPostcodeZip,omitempty"`

	// Security
	CheckValue string `json:"checkValue"`

	// Integration info (REQUIRED)
	IsMobilePayment    int    `json:"isMobilePayment"`
	IntegrationType    string `json:"integrationType"` // Max 20 chars
	IntegrationVersion string `json:"integrationVersion"`
	PackageName        string `json:"packageName"` // REQUIRED for mobile payments
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
	// Billing address (defaults provided if empty)
	BillingStreet   string
	BillingCity     string
	BillingPostcode string
	// Package name for mobile apps
	PackageName string
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
	if lastName == "" {
		lastName = "." // PAYable requires last name
	}

	// Set default email if not provided
	customerEmail := params.CustomerEmail
	if customerEmail == "" {
		customerEmail = "customer@smarttransit.lk" // Default email for PAYable
	}

	// Set default billing address if not provided
	billingStreet := params.BillingStreet
	if billingStreet == "" {
		billingStreet = "Sri Lanka"
	}
	billingCity := params.BillingCity
	if billingCity == "" {
		billingCity = "Colombo"
	}
	billingPostcode := params.BillingPostcode
	if billingPostcode == "" {
		billingPostcode = "00000"
	}

	// Set package name (required for mobile)
	packageName := params.PackageName
	if packageName == "" {
		packageName = "lk.smarttransit.passenger" // Default package name
	}

	// Set default phone if not provided (required by PAYable)
	customerPhone := params.CustomerPhone
	if customerPhone == "" {
		customerPhone = "0770000000" // Default phone for PAYable
	}

	// Get endpoint URL
	endpointURL, ok := PAYableEnvironmentURLs[s.config.Environment]
	if !ok {
		endpointURL = PAYableEnvironmentURLs["sandbox"] // Default to sandbox
	}

	// Build status return URL
	statusReturnURL := fmt.Sprintf("%s/status-view", endpointURL)

	// Build request - NOTE: merchantToken is NOT sent (PAYable rejects it)
	// merchantToken is only used for checkValue calculation
	request := &PAYablePaymentRequest{
		MerchantKey:               s.config.MerchantKey,
		LogoURL:                   s.config.LogoURL,
		ReturnURL:                 s.config.ReturnURL,
		WebhookURL:                s.config.WebhookURL,
		StatusReturnURL:           statusReturnURL,
		PaymentType:               1, // One-time payment
		InvoiceID:                 params.InvoiceID,
		Amount:                    params.Amount,
		CurrencyCode:              params.CurrencyCode,
		OrderDescription:          params.OrderDescription,
		CustomerFirstName:         firstName,
		CustomerLastName:          lastName,
		CustomerEmail:             customerEmail,
		CustomerMobilePhone:       customerPhone,
		BillingAddressStreet:       billingStreet,
		BillingAddressCity:         billingCity,
		BillingAddressCountry:      "LK", // Sri Lanka
		BillingAddressPostcodeZip:  billingPostcode,
		ShippingAddressStreet:      billingStreet,  // Same as billing for bus tickets
		ShippingAddressCity:        billingCity,
		ShippingAddressCountry:     "LK",
		ShippingAddressPostcodeZip: billingPostcode,
		CheckValue:                 checkValue,
		IsMobilePayment:           1,
		IntegrationType:           "SmartTransit", // Max 20 chars
		IntegrationVersion:        "1.0.0",
		PackageName:               packageName,
	}

	s.logger.WithFields(logrus.Fields{
		"invoice_id": params.InvoiceID,
		"amount":     params.Amount,
		"currency":   params.CurrencyCode,
		"endpoint":   endpointURL,
	}).Info("Initiating PAYable payment")

	// Make HTTP request
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the request payload for debugging
	s.logger.WithFields(logrus.Fields{
		"invoice_id":           params.InvoiceID,
		"amount":              params.Amount,
		"currency":            params.CurrencyCode,
		"customer_email":      customerEmail,
		"customer_phone":      customerPhone,
		"billing_country":     "LK",
		"shipping_included":   billingStreet,
		"merchant_key_prefix": s.config.MerchantKey[:10],
		"full_request":        string(jsonBody),
	}).Info("PAYable full request payload")

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
	}).Info("PAYable response received") // Changed from Debug to Info

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("payment gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var paymentResp PAYablePaymentResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		s.logger.WithFields(logrus.Fields{
			"body":  string(body),
			"error": err.Error(),
		}).Error("Failed to parse PAYable response")
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Log parsed response for debugging
	s.logger.WithFields(logrus.Fields{
		"parsed_status":       paymentResp.Status,
		"parsed_uid":          paymentResp.UID,
		"parsed_payment_page": paymentResp.PaymentPage,
		"parsed_message":      paymentResp.Message,
	}).Info("PAYable parsed response")

	// PAYable returns "PENDING" when payment is ready for user, or "success" in some cases
	// Both are valid successful responses
	if paymentResp.Status != "success" && paymentResp.Status != "PENDING" {
		// Try to get more details from the raw response
		errMsg := paymentResp.Message
		if errMsg == "" {
			errMsg = fmt.Sprintf("status=%s, raw=%s", paymentResp.Status, string(body))
		}
		return nil, fmt.Errorf("payment initiation failed: %s", errMsg)
	}

	// Validate we got a payment page URL
	if paymentResp.PaymentPage == "" {
		return nil, fmt.Errorf("payment initiation failed: no payment page URL returned")
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

	// Status check endpoint - use same base URL as payment endpoint
	endpointURL, ok := PAYableEnvironmentURLs[s.config.Environment]
	if !ok {
		endpointURL = PAYableEnvironmentURLs["sandbox"]
	}
	// The check-status endpoint is at the same base URL
	statusURL := strings.Replace(endpointURL, "/ipg/", "/check-status/", 1)

	s.logger.WithFields(logrus.Fields{
		"uid":         uid,
		"status_url":  statusURL,
		"environment": s.config.Environment,
	}).Info("Checking PAYable payment status")

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
