package sms

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DialogURLGateway implements SMS sending using Dialog's GET request API (URL method)
// This method uses an esmsqk key instead of username/password authentication
type DialogURLGateway struct {
	apiKey  string // esmsqk key from Dialog portal
	mask    string // Source address/mask
	appHash string // Optional: App signature hash for SMS auto-read (Android)
}

// NewDialogURLGateway creates a new Dialog URL gateway instance
func NewDialogURLGateway(apiKey, mask, appHash string) *DialogURLGateway {
	return &DialogURLGateway{
		apiKey:  apiKey,
		mask:    mask,
		appHash: appHash,
	}
}

// SendOTP sends an OTP via Dialog's URL-based SMS API
func (d *DialogURLGateway) SendOTP(phone, otpCode string) (int64, error) {
	fmt.Printf("📱 SendOTP (URL method) called - Phone: %s, OTP: %s\n", phone, otpCode)

	// Format phone number for Dialog
	formattedPhone, err := FormatPhoneForDialog(phone)
	if err != nil {
		fmt.Printf("❌ Phone formatting error: %v\n", err)
		return 0, fmt.Errorf("invalid phone number: %v", err)
	}

	fmt.Printf("📞 Formatted phone: %s\n", formattedPhone)

	// Create the message with app hash for Android SMS auto-read
	var message string
	if d.appHash != "" {
		// Format for Android SMS auto-read
		message = fmt.Sprintf("Your SmartTransit OTP is: %s\n\nPlease use the above OTP to complete your action.\n\nRegards,\nSmartTransit\n%s", otpCode, d.appHash)
	} else {
		// Fallback message without app hash
		message = fmt.Sprintf("Your OTP code is: %s. Valid for 5 minutes. Do not share this code.", otpCode)
	}
	fmt.Printf("💬 Message: %s\n", message)

	// Build the URL with query parameters
	baseURL := "https://e-sms.dialog.lk/api/v1/message-via-url/create/url-campaign"

	params := url.Values{}
	params.Add("esmsqk", d.apiKey)
	params.Add("list", formattedPhone)
	params.Add("source_address", d.mask)
	params.Add("message", message)

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	fmt.Printf("🌐 Request URL: %s (with masked API key)\n", strings.Replace(fullURL, d.apiKey, "***MASKED***", 1))

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make the GET request
	fmt.Println("📤 Sending GET request to Dialog...")
	resp, err := client.Get(fullURL)
	if err != nil {
		fmt.Printf("❌ HTTP request error: %v\n", err)
		return 0, fmt.Errorf("failed to send SMS: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Error reading response: %v\n", err)
		return 0, fmt.Errorf("failed to read SMS response: %v", err)
	}

	responseStr := strings.TrimSpace(string(body))
	fmt.Printf("📥 Response status: %d\n", resp.StatusCode)
	fmt.Printf("📄 Response body: %s\n", responseStr)

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Non-200 status code: %d\n", resp.StatusCode)
		return 0, fmt.Errorf("SMS API returned status %d: %s", resp.StatusCode, responseStr)
	}

	// Parse response - Dialog returns "1" for success, or error_id for failure
	if responseStr == "1" {
		// Success - generate a pseudo transaction ID
		transactionID := time.Now().Unix()
		fmt.Printf("✅ SMS sent successfully! Transaction ID: %d\n", transactionID)
		return transactionID, nil
	}

	// Failed - response is an error ID
	fmt.Printf("❌ SMS sending failed with error code: %s\n", responseStr)
	return 0, fmt.Errorf("SMS sending failed with error code: %s", responseStr)
}

// GetName returns the name of this SMS gateway
func (d *DialogURLGateway) GetName() string {
	return "Dialog URL Gateway"
}
