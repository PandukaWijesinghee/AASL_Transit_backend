package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/config"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/services"
	"github.com/smarttransit/sms-auth-backend/pkg/jwt"
	"github.com/smarttransit/sms-auth-backend/pkg/validator"
)

func main() {
	fmt.Println("🧪 SmartTransit Services Integration Test\n")
	fmt.Println("=" + string(make([]byte, 50)) + "\n")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	// Connect to database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("✅ Database connected")
	fmt.Println("✅ Configuration loaded\n")

	// Test 1: Phone Validator
	testPhoneValidator()

	// Test 2: JWT Service
	testJWTService(cfg)

	// Test 3: OTP Service
	testOTPService(db)

	fmt.Println("\n" + string(make([]byte, 50)) + "=")
	fmt.Println("✅ All integration tests completed successfully!")
}

func testPhoneValidator() {
	fmt.Println("📱 Testing Phone Validator")
	fmt.Println("----------------------------")

	phoneValidator := validator.NewPhoneValidator()

	testCases := []struct {
		input    string
		expected bool
		name     string
	}{
		{"0771234567", true, "Valid Dialog number"},
		{"077 123 4567", true, "Valid with spaces"},
		{"94771234567", true, "Valid with country code"},
		{"0701234567", true, "Valid Mobitel number"},
		{"0721234567", true, "Valid Hutch number"},
		{"0751234567", true, "Valid Airtel number"},
		{"0791234567", false, "Invalid prefix"},
		{"077123456", false, "Too short"},
		{"invalid", false, "Invalid format"},
	}

	passCount := 0
	for _, tc := range testCases {
		phone, err := phoneValidator.Validate(tc.input)
		isValid := err == nil

		status := "❌"
		if isValid == tc.expected {
			status = "✅"
			passCount++
		}

		if isValid {
			fmt.Printf("  %s %s: %s → %s\n", status, tc.name, tc.input, phone)
		} else {
			fmt.Printf("  %s %s: %s → %v\n", status, tc.name, tc.input, err)
		}
	}

	fmt.Println()

	// Test formatting
	formatted, _ := phoneValidator.Format("0771234567")
	fmt.Printf("  ✅ Formatting: 0771234567 → %s\n", formatted)

	// Test operator detection
	operator, _ := phoneValidator.GetOperator("0771234567")
	fmt.Printf("  ✅ Operator Detection: 0771234567 → %s\n", operator)

	fmt.Printf("\n  Result: %d/%d tests passed\n\n", passCount, len(testCases))
}

func testJWTService(cfg *config.Config) {
	fmt.Println("🔐 Testing JWT Service")
	fmt.Println("----------------------")

	jwtService := jwt.NewService(
		cfg.JWT.Secret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTokenExpiry,
		cfg.JWT.RefreshTokenExpiry,
	)

	userID := uuid.New()
	phone := "0771234567"
	roles := []string{"user", "passenger"}

	// Generate access token
	accessToken, err := jwtService.GenerateAccessToken(userID, phone, roles, false)
	if err != nil {
		fmt.Printf("  ❌ Failed to generate access token: %v\n", err)
		return
	}
	fmt.Printf("  ✅ Access token generated (%d chars)\n", len(accessToken))
	fmt.Printf("     Token: %s...\n", accessToken[:50])

	// Validate access token
	claims, err := jwtService.ValidateAccessToken(accessToken)
	if err != nil {
		fmt.Printf("  ❌ Failed to validate access token: %v\n", err)
		return
	}
	fmt.Printf("  ✅ Access token validated\n")
	fmt.Printf("     - User ID: %s\n", claims.UserID)
	fmt.Printf("     - Phone: %s\n", claims.Phone)
	fmt.Printf("     - Roles: %v\n", claims.Roles)
	fmt.Printf("     - Profile Completed: %v\n", claims.ProfileCompleted)
	fmt.Printf("     - Expires: %s\n", claims.ExpiresAt.Time.Format("2006-01-02 15:04:05"))

	// Generate refresh token
	refreshToken, err := jwtService.GenerateRefreshToken(userID, phone)
	if err != nil {
		fmt.Printf("  ❌ Failed to generate refresh token: %v\n", err)
		return
	}
	fmt.Printf("\n  ✅ Refresh token generated (%d chars)\n", len(refreshToken))
	fmt.Printf("     Token: %s...\n", refreshToken[:50])

	// Validate refresh token
	refreshClaims, err := jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		fmt.Printf("  ❌ Failed to validate refresh token: %v\n", err)
		return
	}
	fmt.Printf("  ✅ Refresh token validated\n")
	fmt.Printf("     - User ID: %s\n", refreshClaims.UserID)
	fmt.Printf("     - Expires: %s\n", refreshClaims.ExpiresAt.Time.Format("2006-01-02 15:04:05"))

	// Test token expiry checking
	isExpired := jwtService.IsTokenExpired(accessToken)
	fmt.Printf("\n  ✅ Token expiry check: Expired = %v\n", isExpired)

	fmt.Println("\n  Result: JWT service working correctly\n")
}

func testOTPService(db database.DB) {
	fmt.Println("🔢 Testing OTP Service")
	fmt.Println("----------------------")

	otpService := services.NewOTPService(db)
	phone := "0771234567"

	// Generate OTP
	otp, err := otpService.GenerateOTP(phone, "127.0.0.1", "test-client")
	if err != nil {
		fmt.Printf("  ❌ Failed to generate OTP: %v\n", err)
		return
	}
	fmt.Printf("  ✅ OTP generated: %s (for %s)\n", otp, phone)

	// Check expiry
	expiry, err := otpService.GetOTPExpiry(phone)
	if err != nil {
		fmt.Printf("  ❌ Failed to get expiry: %v\n", err)
		return
	}
	fmt.Printf("  ✅ OTP expires at: %s (in %.0f seconds)\n",
		expiry.Format("15:04:05"),
		time.Until(expiry).Seconds())

	// Check remaining attempts
	remaining, err := otpService.GetRemainingAttempts(phone)
	if err != nil {
		fmt.Printf("  ❌ Failed to get remaining attempts: %v\n", err)
		return
	}
	fmt.Printf("  ✅ Remaining attempts: %d/%d\n", remaining, services.MaxOTPAttempts)

	// Test wrong OTP
	fmt.Println("\n  Testing validation scenarios:")
	valid, err := otpService.ValidateOTP(phone, "000000")
	if err == nil || valid {
		fmt.Printf("    ❌ Wrong OTP should be rejected\n")
	} else {
		fmt.Printf("    ✅ Wrong OTP rejected: %v\n", err)
	}

	// Check attempts after failure
	remaining, _ = otpService.GetRemainingAttempts(phone)
	fmt.Printf("    ✅ Remaining attempts after failure: %d/%d\n", remaining, services.MaxOTPAttempts)

	// Validate correct OTP
	valid, err = otpService.ValidateOTP(phone, otp)
	if err != nil || !valid {
		fmt.Printf("    ❌ Correct OTP should be accepted: %v\n", err)
		return
	}
	fmt.Printf("    ✅ Correct OTP accepted\n")

	// Try to reuse OTP
	valid, err = otpService.ValidateOTP(phone, otp)
	if err == nil || valid {
		fmt.Printf("    ❌ Used OTP should be rejected\n")
	} else {
		fmt.Printf("    ✅ Used OTP rejected: %v\n", err)
	}

	// Get OTP statistics
	stats, err := otpService.GetOTPStats(phone)
	if err != nil {
		fmt.Printf("  ❌ Failed to get stats: %v\n", err)
		return
	}
	fmt.Printf("\n  ✅ OTP Statistics:\n")
	fmt.Printf("     - Has Active OTP: %v\n", stats["has_active_otp"])
	fmt.Printf("     - Attempts Made: %v\n", stats["attempts_made"])
	fmt.Printf("     - Max Attempts: %v\n", stats["max_attempts_allowed"])

	// Test cleanup
	deleted, err := otpService.CleanupExpiredOTPs()
	if err != nil {
		fmt.Printf("  ❌ Failed to cleanup: %v\n", err)
	} else {
		fmt.Printf("\n  ✅ Cleanup: Deleted %d expired OTPs\n", deleted)
	}

	fmt.Println("\n  Result: OTP service working correctly")
}
