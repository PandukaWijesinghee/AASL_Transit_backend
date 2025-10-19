package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DialogGateway implements SMS sending via Dialog eSMS API
type DialogGateway struct {
	apiURL   string
	username string
	password string
	mask     string
	client   *http.Client

	// Token management
	token       string
	tokenMutex  sync.RWMutex
	tokenExpiry time.Time

	// SMS Auto-read (Android)
	appHash string // 11-character hash for SMS auto-read feature
}

// DialogConfig holds configuration for Dialog SMS Gateway
type DialogConfig struct {
	APIURL   string
	Username string
	Password string
	Mask     string
	AppHash  string // Optional: App signature hash for SMS auto-read (Android)
}

// NewDialogGateway creates a new Dialog SMS Gateway client
func NewDialogGateway(config DialogConfig) *DialogGateway {
	return &DialogGateway{
		apiURL:   config.APIURL,
		username: config.Username,
		password: config.Password,
		mask:     config.Mask,
		appHash:  config.AppHash,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoginRequest represents the login request structure
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the login response structure
type LoginResponse struct {
	Status     string      `json:"status"`
	Comment    string      `json:"comment"`
	Token      string      `json:"token"`
	Expiration int         `json:"expiration"` // Token expiry in seconds
	UserData   interface{} `json:"userData"`
	ErrCode    string      `json:"errCode"`
}

// SMSRecipient represents a single SMS recipient
type SMSRecipient struct {
	Mobile string `json:"mobile"`
}

// SendSMSRequest represents the SMS sending request structure
type SendSMSRequest struct {
	MSISDN        []SMSRecipient `json:"msisdn"`
	Message       string         `json:"message"`
	SourceAddress string         `json:"sourceAddress,omitempty"`
	TransactionID int64          `json:"transaction_id"`
	PaymentMethod int            `json:"payment_method,omitempty"` // 0 = wallet, 4 = package
}

// SendSMSResponse represents the SMS sending response structure
type SendSMSResponse struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
	Data    struct {
		CampaignID         int     `json:"campaignId"`
		CampaignCost       float64 `json:"campaignCost"`
		WalletBalance      float64 `json:"walletBalance"`
		DuplicatesRemoved  int     `json:"duplicatesRemoved"`
		InvalidNumbers     int     `json:"invalidNumbers"`
		MaskBlockedNumbers int     `json:"mask_blocked_numbers"`
	} `json:"data"`
	ErrCode string `json:"errCode"`
}

// CheckStatusRequest represents campaign status check request
type CheckStatusRequest struct {
	TransactionID int64 `json:"transaction_id"`
}

// CheckStatusResponse represents campaign status check response
type CheckStatusResponse struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
	Data    struct {
		CampaignStatus string `json:"campaign_status"` // pending, running, completed
	} `json:"data"`
	ErrCode       string `json:"errCode"`
	TransactionID int64  `json:"transaction_id"`
}

// GetAccessToken logs in and retrieves an access token
func (d *DialogGateway) GetAccessToken() error {
	fmt.Println("🔐 Attempting Dialog API login...")

	loginReq := LoginRequest{
		Username: d.username,
		Password: d.password,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	url := fmt.Sprintf("%s/login", d.apiURL)
	fmt.Printf("🌐 Login URL: %s\n", url)
	fmt.Printf("👤 Username: %s\n", d.username)
	fmt.Printf("🔑 Password: %s (length: %d)\n", strings.Repeat("*", len(d.password)), len(d.password))

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		fmt.Printf("❌ HTTP request failed: %v\n", err)
		return fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("📥 Login response status: %d\n", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	fmt.Printf("📄 Login response body: %s\n", string(body))

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResp.Status != "success" {
		return fmt.Errorf("login failed: %s (error code: %s)", loginResp.Comment, loginResp.ErrCode)
	}

	// Store token with expiry
	d.tokenMutex.Lock()
	d.token = loginResp.Token
	d.tokenExpiry = time.Now().Add(time.Duration(loginResp.Expiration) * time.Second)
	d.tokenMutex.Unlock()

	return nil
}

// isTokenValid checks if the current token is still valid
func (d *DialogGateway) isTokenValid() bool {
	d.tokenMutex.RLock()
	defer d.tokenMutex.RUnlock()

	if d.token == "" {
		return false
	}

	// Consider token invalid 5 minutes before actual expiry
	return time.Now().Before(d.tokenExpiry.Add(-5 * time.Minute))
}

// ensureValidToken ensures we have a valid access token
func (d *DialogGateway) ensureValidToken() error {
	if d.isTokenValid() {
		return nil
	}

	return d.GetAccessToken()
}

// FormatPhoneForDialog converts phone number to Dialog's 9-digit format
// Input: "0771234567" (10 digits) or "94771234567" (11 digits) or "+94771234567"
// Output: "771234567" (9 digits without prefix)
func FormatPhoneForDialog(phone string) (string, error) {
	// Remove all non-digits
	re := regexp.MustCompile(`[^0-9]`)
	phone = re.ReplaceAllString(phone, "")

	// Remove country code if present
	if strings.HasPrefix(phone, "94") && len(phone) == 11 {
		phone = phone[2:] // Remove "94"
	}

	// Remove leading 0 if present
	if strings.HasPrefix(phone, "0") && len(phone) == 10 {
		phone = phone[1:] // Remove "0"
	}

	// Validate final length
	if len(phone) != 9 {
		return "", fmt.Errorf("invalid phone number length after formatting: %d digits (expected 9)", len(phone))
	}

	// Validate Sri Lankan mobile prefix (07X)
	if !strings.HasPrefix(phone, "7") {
		return "", fmt.Errorf("invalid Sri Lankan mobile prefix: must start with 7")
	}

	return phone, nil
}

// SendOTP sends an OTP to a single phone number
func (d *DialogGateway) SendOTP(phone string, otpCode string) (int64, error) {
	fmt.Printf("📱 SendOTP called - Phone: %s, OTP: %s\n", phone, otpCode)

	// Ensure we have a valid token
	fmt.Println("🔑 Checking access token...")
	if err := d.ensureValidToken(); err != nil {
		fmt.Printf("❌ Token error: %v\n", err)
		return 0, fmt.Errorf("failed to get access token: %w", err)
	}
	fmt.Println("✅ Access token valid")

	// Format phone number for Dialog (9-digit format)
	fmt.Printf("📞 Formatting phone: %s\n", phone)
	formattedPhone, err := FormatPhoneForDialog(phone)
	if err != nil {
		fmt.Printf("❌ Phone format error: %v\n", err)
		return 0, fmt.Errorf("failed to format phone number: %w", err)
	}
	fmt.Printf("✅ Phone formatted: %s -> %s\n", phone, formattedPhone)

	// Generate unique transaction ID (timestamp in microseconds)
	transactionID := time.Now().UnixMicro()

	// Prepare SMS message with app hash for Android SMS auto-read
	var message string
	if d.appHash != "" {
		// Format for Android SMS auto-read:
		// OTP code followed by message and app hash on a new line
		message = fmt.Sprintf("Your SmartTransit OTP is: %s\n\nPlease use the above OTP to complete your action.\n\nRegards,\nSmartTransit\n%s", otpCode, d.appHash)
	} else {
		// Fallback message without app hash
		message = fmt.Sprintf("Your OTP is %s. Valid for 5 minutes. Do not share this code with anyone.", otpCode)
	}

	// Prepare request
	smsReq := SendSMSRequest{
		MSISDN: []SMSRecipient{
			{Mobile: formattedPhone},
		},
		Message:       message,
		SourceAddress: d.mask,
		TransactionID: transactionID,
		PaymentMethod: 0, // 0 = wallet payment
	}

	jsonData, err := json.Marshal(smsReq)
	if err != nil {
		fmt.Printf("❌ Marshal error: %v\n", err)
		return 0, fmt.Errorf("failed to marshal SMS request: %w", err)
	}
	fmt.Printf("📤 SMS Request: %s\n", string(jsonData))

	url := fmt.Sprintf("%s/sms", d.apiURL)
	fmt.Printf("🌐 API URL: %s\n", url)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ Request creation error: %v\n", err)
		return 0, fmt.Errorf("failed to create SMS request: %w", err)
	}

	// Add headers
	d.tokenMutex.RLock()
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.token))
	d.tokenMutex.RUnlock()
	req.Header.Set("Content-Type", "application/json")
	fmt.Println("✅ Headers set, sending request...")

	// Send request
	resp, err := d.client.Do(req)
	if err != nil {
		fmt.Printf("❌ HTTP request error: %v\n", err)
		return 0, fmt.Errorf("failed to send SMS request: %w", err)
	}
	defer resp.Body.Close()
	fmt.Printf("📥 HTTP Status: %d\n", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Response read error: %v\n", err)
		return 0, fmt.Errorf("failed to read SMS response: %w", err)
	}
	fmt.Printf("📄 Response Body: %s\n", string(body))

	var smsResp SendSMSResponse
	if err := json.Unmarshal(body, &smsResp); err != nil {
		fmt.Printf("❌ Response parse error: %v\n", err)
		return 0, fmt.Errorf("failed to parse SMS response: %w", err)
	}

	if smsResp.Status != "success" {
		fmt.Printf("❌ Dialog API Error - Status: %s, Comment: %s, ErrCode: %s\n",
			smsResp.Status, smsResp.Comment, smsResp.ErrCode)
		return 0, fmt.Errorf("SMS sending failed: %s (error code: %s)", smsResp.Comment, smsResp.ErrCode)
	}

	fmt.Printf("✅ SMS sent successfully! Campaign ID: %d, Cost: %.2f\n",
		smsResp.Data.CampaignID, smsResp.Data.CampaignCost)
	return transactionID, nil
}

// CheckCampaignStatus checks the status of an SMS campaign
func (d *DialogGateway) CheckCampaignStatus(transactionID int64) (string, error) {
	// Ensure we have a valid token
	if err := d.ensureValidToken(); err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Prepare request
	checkReq := CheckStatusRequest{
		TransactionID: transactionID,
	}

	jsonData, err := json.Marshal(checkReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal status check request: %w", err)
	}

	url := fmt.Sprintf("%s/sms/check-transaction", d.apiURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create status check request: %w", err)
	}

	// Add headers
	d.tokenMutex.RLock()
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.token))
	d.tokenMutex.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send status check request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read status check response: %w", err)
	}

	var checkResp CheckStatusResponse
	if err := json.Unmarshal(body, &checkResp); err != nil {
		return "", fmt.Errorf("failed to parse status check response: %w", err)
	}

	if checkResp.Status != "success" {
		return "", fmt.Errorf("status check failed: %s (error code: %s)", checkResp.Comment, checkResp.ErrCode)
	}

	return checkResp.Data.CampaignStatus, nil
}

// SendBulkSMS sends SMS to multiple recipients (max 1000 recommended)
func (d *DialogGateway) SendBulkSMS(phones []string, message string) (int64, error) {
	// Ensure we have a valid token
	if err := d.ensureValidToken(); err != nil {
		return 0, fmt.Errorf("failed to get access token: %w", err)
	}

	// Format all phone numbers
	recipients := make([]SMSRecipient, 0, len(phones))
	for _, phone := range phones {
		formattedPhone, err := FormatPhoneForDialog(phone)
		if err != nil {
			// Skip invalid numbers but continue
			continue
		}
		recipients = append(recipients, SMSRecipient{Mobile: formattedPhone})
	}

	if len(recipients) == 0 {
		return 0, fmt.Errorf("no valid recipients after formatting")
	}

	// Generate unique transaction ID
	transactionID := time.Now().UnixMicro()

	// Prepare request
	smsReq := SendSMSRequest{
		MSISDN:        recipients,
		Message:       message,
		SourceAddress: d.mask,
		TransactionID: transactionID,
		PaymentMethod: 0,
	}

	jsonData, err := json.Marshal(smsReq)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal SMS request: %w", err)
	}

	url := fmt.Sprintf("%s/sms", d.apiURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to create SMS request: %w", err)
	}

	// Add headers
	d.tokenMutex.RLock()
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.token))
	d.tokenMutex.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send SMS request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read SMS response: %w", err)
	}

	var smsResp SendSMSResponse
	if err := json.Unmarshal(body, &smsResp); err != nil {
		return 0, fmt.Errorf("failed to parse SMS response: %w", err)
	}

	if smsResp.Status != "success" {
		return 0, fmt.Errorf("SMS sending failed: %s (error code: %s)", smsResp.Comment, smsResp.ErrCode)
	}

	return transactionID, nil
}

// GetName returns the name of this SMS gateway
func (d *DialogGateway) GetName() string {
	return "Dialog API v2 Gateway"
}
