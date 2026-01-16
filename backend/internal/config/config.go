package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	// Server configuration
	Server ServerConfig

	// Database configuration
	Database DatabaseConfig

	// JWT configuration
	JWT JWTConfig

	// SMS configuration
	SMS SMSConfig

	// OTP configuration
	OTP OTPConfig

	// Rate limiting configuration
	RateLimit RateLimitConfig

	// CORS configuration
	CORS CORSConfig

	// Security configuration
	Security SecurityConfig

	// Payment gateway configuration
	Payment PaymentConfig
}

// PaymentConfig holds PAYable IPG configuration
type PaymentConfig struct {
	Environment   string // "sandbox" or "production"
	MerchantKey   string // PAYable merchant key
	MerchantToken string // PAYable merchant token (SECRET - never expose to client)
	LogoURL       string // Merchant logo URL for payment page
	ReturnURL     string // URL to redirect after payment (app deep link)
	WebhookURL    string // Server webhook URL for payment notifications
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port        string
	Environment string // development, staging, production
	LogLevel    string // debug, info, warn, error
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	URL                string
	MaxConnections     int
	MaxIdleConnections int
	ConnMaxLifetime    time.Duration
}

// JWTConfig holds JWT-related configuration
type JWTConfig struct {
	Secret             string
	RefreshSecret      string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

// SMSConfig holds SMS gateway configuration
type SMSConfig struct {
	Mode             string // "dev" or "production" - dev returns OTP in response, production sends actual SMS
	Method           string // "url" or "api_v2" - url uses GET with esmsqk, api_v2 uses POST with login
	APIURL           string
	APIKey           string
	ESMSQK           string // Dialog URL message key (for URL method)
	Username         string
	Password         string
	SenderID         string
	Mask             string // Dialog SMS mask/source address
	DriverAppHash    string // App signature hash for Driver/Conductor app SMS auto-read (Android)
	PassengerAppHash string // App signature hash for Passenger app SMS auto-read (Android)
}

// OTPConfig holds OTP-related configuration
type OTPConfig struct {
	Length            int
	ExpiryMinutes     int
	MaxAttempts       int
	RateLimit         int
	RateWindowMinutes int
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Requests      int
	WindowSeconds int
}

// CORSConfig holds CORS-related configuration
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	BcryptCost       int
	EnableRequestLog bool
	EnableAuditLog   bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := &Config{
		Server: ServerConfig{
			Port:        getEnv("PORT", "8080"),
			Environment: getEnv("ENVIRONMENT", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
		},
		Database: DatabaseConfig{
			URL:                getEnv("DATABASE_URL", ""),
			MaxConnections:     getEnvAsInt("DATABASE_MAX_CONNECTIONS", 10),
			MaxIdleConnections: getEnvAsInt("DATABASE_MAX_IDLE_CONNECTIONS", 5),
			ConnMaxLifetime:    time.Duration(getEnvAsInt("DATABASE_CONN_MAX_LIFETIME", 300)) * time.Second,
		},
		JWT: JWTConfig{
			Secret:             getEnv("JWT_SECRET", ""),
			RefreshSecret:      getEnv("JWT_REFRESH_SECRET", ""),
			AccessTokenExpiry:  time.Duration(getEnvAsInt("JWT_ACCESS_TOKEN_EXPIRY", 3600)) * time.Second,
			RefreshTokenExpiry: time.Duration(getEnvAsInt("JWT_REFRESH_TOKEN_EXPIRY", 604800)) * time.Second,
		},
		SMS: SMSConfig{
			Mode:             getEnv("SMS_MODE", "dev"),          // "dev" or "production"
			Method:           getEnv("DIALOG_SMS_METHOD", "url"), // "url" or "api_v2"
			APIURL:           getEnv("DIALOG_SMS_API_URL", "https://e-sms.dialog.lk/api/v2"),
			ESMSQK:           getEnv("DIALOG_SMS_ESMSQK", ""),
			Username:         getEnv("DIALOG_SMS_USERNAME", ""),
			Password:         getEnv("DIALOG_SMS_PASSWORD", ""),
			Mask:             getEnv("DIALOG_SMS_MASK", ""),
			DriverAppHash:    getEnv("DRIVER_APP_HASH", ""),    // SMS auto-read for driver app
			PassengerAppHash: getEnv("PASSENGER_APP_HASH", ""), // SMS auto-read for passenger app
			// Deprecated fields kept for backward compatibility
			APIKey:   getEnv("DIALOG_SMS_API_KEY", ""),
			SenderID: getEnv("DIALOG_SMS_SENDER_ID", "SmartTransit"),
		},
		OTP: OTPConfig{
			Length:            getEnvAsInt("OTP_LENGTH", 6),
			ExpiryMinutes:     getEnvAsInt("OTP_EXPIRY_MINUTES", 5),
			MaxAttempts:       getEnvAsInt("OTP_MAX_ATTEMPTS", 3),
			RateLimit:         getEnvAsInt("OTP_RATE_LIMIT", 3),
			RateWindowMinutes: getEnvAsInt("OTP_RATE_WINDOW_MINUTES", 10),
		},
		RateLimit: RateLimitConfig{
			Requests:      getEnvAsInt("RATE_LIMIT_REQUESTS", 100),
			WindowSeconds: getEnvAsInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowedMethods: getEnvAsSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			AllowedHeaders: getEnvAsSlice("CORS_ALLOWED_HEADERS", []string{"Content-Type", "Authorization"}),
		},
		Security: SecurityConfig{
			BcryptCost:       getEnvAsInt("BCRYPT_COST", 12),
			EnableRequestLog: getEnvAsBool("ENABLE_REQUEST_LOGGING", true),
			EnableAuditLog:   getEnvAsBool("ENABLE_AUDIT_LOGGING", true),
		},
		Payment: PaymentConfig{
			Environment:   getEnv("PAYABLE_ENVIRONMENT", "sandbox"),
			MerchantKey:   getEnv("PAYABLE_MERCHANT_KEY", ""),
			MerchantToken: getEnv("PAYABLE_MERCHANT_TOKEN", ""),
			LogoURL:       getEnv("PAYABLE_LOGO_URL", ""),
			ReturnURL:     getEnv("PAYABLE_RETURN_URL", ""),
			WebhookURL:    getEnv("PAYABLE_WEBHOOK_URL", ""),
		},
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if c.JWT.RefreshSecret == "" {
		return fmt.Errorf("JWT_REFRESH_SECRET is required")
	}

	// Validate SMS configuration only in production mode
	if c.SMS.Mode == "production" {
		// Check which method is being used
		if c.SMS.Method == "url" {
			// URL method requires ESMSQK key
			if c.SMS.ESMSQK == "" {
				return fmt.Errorf("DIALOG_SMS_ESMSQK is required for URL method in production mode")
			}
		} else if c.SMS.Method == "api_v2" {
			// API v2 method requires username and password
			if c.SMS.APIURL == "" {
				return fmt.Errorf("DIALOG_SMS_API_URL is required for API v2 method in production mode")
			}

			if c.SMS.Username == "" {
				return fmt.Errorf("DIALOG_SMS_USERNAME is required for API v2 method in production mode")
			}

			if c.SMS.Password == "" {
				return fmt.Errorf("DIALOG_SMS_PASSWORD is required for API v2 method in production mode")
			}
		} else {
			return fmt.Errorf("invalid SMS method: %s (must be 'url' or 'api_v2')", c.SMS.Method)
		}
	}

	return nil
}

// Helper functions to get environment variables

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Invalid integer value for %s, using default: %d", key, defaultValue)
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		log.Printf("Invalid boolean value for %s, using default: %t", key, defaultValue)
		return defaultValue
	}
	return value
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	// Split by comma
	var result []string
	for _, v := range splitString(valueStr, ",") {
		trimmed := trimString(v)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}

// Helper to split strings
func splitString(s, sep string) []string {
	var result []string
	current := ""
	for _, char := range s {
		if string(char) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// Helper to trim strings
func trimString(s string) string {
	start := 0
	end := len(s)

	// Trim leading spaces
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	// Trim trailing spaces
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
