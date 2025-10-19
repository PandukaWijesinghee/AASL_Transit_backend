package sms

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDialogGateway(t *testing.T) {
	config := DialogConfig{
		APIURL:   "https://e-sms.dialog.lk/api/v2",
		Username: "testuser",
		Password: "testpass",
		Mask:     "TestMask",
	}

	gateway := NewDialogGateway(config)

	assert.NotNil(t, gateway)
	assert.Equal(t, config.APIURL, gateway.apiURL)
	assert.Equal(t, config.Username, gateway.username)
	assert.Equal(t, config.Password, gateway.password)
	assert.Equal(t, config.Mask, gateway.mask)
	assert.NotNil(t, gateway.client)
}

func TestFormatPhoneForDialog(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "10-digit format with leading 0",
			input:       "0771234567",
			expected:    "771234567",
			expectError: false,
		},
		{
			name:        "11-digit format with country code 94",
			input:       "94771234567",
			expected:    "771234567",
			expectError: false,
		},
		{
			name:        "12-digit format with +94",
			input:       "+94771234567",
			expected:    "771234567",
			expectError: false,
		},
		{
			name:        "Already 9-digit format",
			input:       "771234567",
			expected:    "771234567",
			expectError: false,
		},
		{
			name:        "With spaces",
			input:       "077 123 4567",
			expected:    "771234567",
			expectError: false,
		},
		{
			name:        "With dashes",
			input:       "077-123-4567",
			expected:    "771234567",
			expectError: false,
		},
		{
			name:        "Mobitel 070",
			input:       "0701234567",
			expected:    "701234567",
			expectError: false,
		},
		{
			name:        "Hutch 072",
			input:       "0721234567",
			expected:    "721234567",
			expectError: false,
		},
		{
			name:        "Airtel 075",
			input:       "0751234567",
			expected:    "751234567",
			expectError: false,
		},
		{
			name:        "Dialog 076",
			input:       "0761234567",
			expected:    "761234567",
			expectError: false,
		},
		{
			name:        "Hutch 078",
			input:       "0781234567",
			expected:    "781234567",
			expectError: false,
		},
		{
			name:        "Invalid - too short",
			input:       "077123",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid - too long",
			input:       "0771234567890",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid - wrong prefix (not 7)",
			input:       "0611234567",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid - landline",
			input:       "0112345678",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatPhoneForDialog(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatPhoneForDialog_AllOperators(t *testing.T) {
	// Test all valid Sri Lankan mobile prefixes
	validPrefixes := []string{"070", "071", "072", "075", "076", "077", "078"}

	for _, prefix := range validPrefixes {
		t.Run("Prefix_"+prefix, func(t *testing.T) {
			phone := "0" + prefix[1:] + "1234567" // e.g., "0701234567"
			result, err := FormatPhoneForDialog(phone)

			require.NoError(t, err)
			assert.Len(t, result, 9)
			assert.True(t, result[0] == '7', "Should start with 7")
		})
	}
}

func TestFormatPhoneForDialog_CountryCodeVariations(t *testing.T) {
	variations := []string{
		"94771234567",    // Without +
		"+94771234567",   // With +
		"94 77 123 4567", // With spaces
		"94-77-123-4567", // With dashes
	}

	for _, phone := range variations {
		t.Run(phone, func(t *testing.T) {
			result, err := FormatPhoneForDialog(phone)

			require.NoError(t, err)
			assert.Equal(t, "771234567", result)
		})
	}
}

func TestDialogGateway_TokenManagement(t *testing.T) {
	config := DialogConfig{
		APIURL:   "https://e-sms.dialog.lk/api/v2",
		Username: "testuser",
		Password: "testpass",
		Mask:     "TestMask",
	}

	gateway := NewDialogGateway(config)

	// Initially no token
	assert.False(t, gateway.isTokenValid())
	assert.Empty(t, gateway.token)

	// Note: Actual token retrieval requires real API credentials
	// This test only verifies the structure
}

func TestDialogConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config DialogConfig
		valid  bool
	}{
		{
			name: "Valid config",
			config: DialogConfig{
				APIURL:   "https://e-sms.dialog.lk/api/v2",
				Username: "kanchanadesilva",
				Password: "Dialog@123",
				Mask:     "KanchTest",
			},
			valid: true,
		},
		{
			name: "Empty API URL",
			config: DialogConfig{
				APIURL:   "",
				Username: "user",
				Password: "pass",
				Mask:     "mask",
			},
			valid: false,
		},
		{
			name: "Empty username",
			config: DialogConfig{
				APIURL:   "https://api.example.com",
				Username: "",
				Password: "pass",
				Mask:     "mask",
			},
			valid: false,
		},
		{
			name: "Empty password",
			config: DialogConfig{
				APIURL:   "https://api.example.com",
				Username: "user",
				Password: "",
				Mask:     "mask",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := NewDialogGateway(tt.config)
			assert.NotNil(t, gateway)

			// Check if essential fields are set
			if tt.valid {
				assert.NotEmpty(t, gateway.apiURL)
				assert.NotEmpty(t, gateway.username)
				assert.NotEmpty(t, gateway.password)
			}
		})
	}
}

func TestFormatPhoneForDialog_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Only spaces",
			input:       "   ",
			expectError: true,
		},
		{
			name:        "Special characters only",
			input:       "+-() ",
			expectError: true,
		},
		{
			name:        "Letters in number",
			input:       "077ABC4567",
			expectError: true,
		},
		{
			name:        "Multiple country codes",
			input:       "949477123456",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FormatPhoneForDialog(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration test - only run with actual Dialog credentials
// Run with: go test -tags=integration
func TestDialogGateway_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// These tests require actual Dialog API credentials
	// Uncomment and add real credentials to test

	/*
		config := DialogConfig{
			APIURL:   "https://e-sms.dialog.lk/api/v2",
			Username: "kanchanadesilva",
			Password: "Dialog@123",
			Mask:     "KanchTest",
		}

		gateway := NewDialogGateway(config)

		// Test 1: Get Access Token
		t.Run("GetAccessToken", func(t *testing.T) {
			err := gateway.GetAccessToken()
			require.NoError(t, err)
			assert.NotEmpty(t, gateway.token)
			assert.True(t, gateway.isTokenValid())
		})

		// Test 2: Send OTP (uncomment only when ready to send actual SMS)
		// t.Run("SendOTP", func(t *testing.T) {
		// 	transactionID, err := gateway.SendOTP("0771234567", "123456")
		// 	require.NoError(t, err)
		// 	assert.Greater(t, transactionID, int64(0))
		// })

		// Test 3: Check Campaign Status
		// t.Run("CheckCampaignStatus", func(t *testing.T) {
		// 	// Use transaction ID from previous test
		// 	status, err := gateway.CheckCampaignStatus(transactionID)
		// 	require.NoError(t, err)
		// 	assert.Contains(t, []string{"pending", "running", "completed"}, status)
		// })
	*/
}
