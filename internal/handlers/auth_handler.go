package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/config"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
	"github.com/smarttransit/sms-auth-backend/internal/utils"
	"github.com/smarttransit/sms-auth-backend/pkg/jwt"
	"github.com/smarttransit/sms-auth-backend/pkg/sms"
	"github.com/smarttransit/sms-auth-backend/pkg/validator"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	jwtService             *jwt.Service
	otpService             *services.OTPService
	phoneValidator         *validator.PhoneValidator
	rateLimitService       *services.RateLimitService
	auditService           *services.AuditService
	userRepository         *database.UserRepository
	refreshTokenRepository *database.RefreshTokenRepository
	smsGateway             sms.SMSGateway
	config                 *config.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	jwtService *jwt.Service,
	otpService *services.OTPService,
	phoneValidator *validator.PhoneValidator,
	rateLimitService *services.RateLimitService,
	auditService *services.AuditService,
	userRepository *database.UserRepository,
	refreshTokenRepository *database.RefreshTokenRepository,
	smsGateway sms.SMSGateway,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		jwtService:             jwtService,
		otpService:             otpService,
		phoneValidator:         phoneValidator,
		rateLimitService:       rateLimitService,
		auditService:           auditService,
		userRepository:         userRepository,
		refreshTokenRepository: refreshTokenRepository,
		smsGateway:             smsGateway,
		config:                 cfg,
	}
}

// SendOTPRequest represents the request to send OTP
type SendOTPRequest struct {
	Phone string `json:"phone_number" binding:"required"`
}

// SendOTPResponse represents the response after sending OTP
type SendOTPResponse struct {
	Message   string    `json:"message"`
	Phone     string    `json:"phone"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int       `json:"expires_in_seconds"`
}

// VerifyOTPRequest represents the request to verify OTP
type VerifyOTPRequest struct {
	Phone string `json:"phone_number" binding:"required"`
	OTP   string `json:"otp" binding:"required"`
}

// VerifyOTPResponse represents the response after verifying OTP
type VerifyOTPResponse struct {
	Message         string   `json:"message"`
	AccessToken     string   `json:"access_token"`
	RefreshToken    string   `json:"refresh_token"`
	ExpiresIn       int      `json:"expires_in_seconds"`
	IsNewUser       bool     `json:"is_new_user"`
	ProfileComplete bool     `json:"profile_complete"`
	Roles           []string `json:"roles"` // User's roles (can be empty for new staff users)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// SendOTP handles POST /api/v1/auth/send-otp
func (h *AuthHandler) SendOTP(c *gin.Context) {
	var req SendOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body",
		})
		return
	}

	// Validate phone number
	phone, err := h.phoneValidator.Validate(req.Phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_phone",
			Message: err.Error(),
		})
		return
	}

	// Get real client IP and user agent
	clientIP := utils.GetRealIP(c)
	userAgent := utils.GetUserAgent(c)

	// Check rate limiting
	if err := h.rateLimitService.CheckOTPRateLimit(phone, clientIP); err != nil {
		if rateLimitErr, ok := err.(*services.RateLimitError); ok {
			// Log rate limit violation
			h.auditService.LogRateLimitViolation(phone, clientIP, userAgent, rateLimitErr.Type, rateLimitErr.RetryAfter)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     rateLimitErr.Message,
				"retry_after": rateLimitErr.RetryAfter,
				"type":        rateLimitErr.Type,
			})
			return
		}
		// Other errors
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "rate_limit_check_failed",
			Message: "Failed to check rate limit",
		})
		return
	}

	// Generate OTP with IP and user agent tracking
	otp, err := h.otpService.GenerateOTP(phone, clientIP, userAgent)
	if err != nil {
		// Log failed OTP request
		h.auditService.LogOTPRequest(phone, clientIP, userAgent, false, "generation_failed")

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "otp_generation_failed",
			Message: "Failed to generate OTP",
		})
		return
	}

	// Record rate limit request
	if err := h.rateLimitService.RecordOTPRequest(phone, clientIP); err != nil {
		// Log error but don't fail the request
		// The OTP is already generated and stored
		c.Error(err) // This logs the error in Gin
	}

	// Log successful OTP request
	h.auditService.LogOTPRequest(phone, clientIP, userAgent, true, "")

	// Get expiry time
	expiresAt, _ := h.otpService.GetOTPExpiry(phone)
	expiresIn := int(time.Until(expiresAt).Seconds())

	// Send SMS based on mode
	if h.config.SMS.Mode == "production" {
		// Production mode: Send actual SMS via Dialog gateway
		log.Printf("🔵 Attempting to send SMS to %s via Dialog gateway...", phone)
		transactionID, err := h.smsGateway.SendOTP(phone, otp)
		if err != nil {
			log.Printf("❌ ERROR: Failed to send SMS to %s: %v", phone, err)
			log.Printf("❌ Error type: %T", err)
			log.Printf("❌ Full error details: %+v", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "sms_send_failed",
				Message: "Failed to send OTP via SMS. Please try again.",
			})
			return
		}

		log.Printf("✅ SMS sent successfully to %s, transaction_id: %d", phone, transactionID)

		// Production response (without OTP)
		c.JSON(http.StatusOK, SendOTPResponse{
			Message:   "OTP sent successfully to your phone",
			Phone:     phone,
			ExpiresAt: expiresAt,
			ExpiresIn: expiresIn,
		})
		return
	}

	// Development mode: Return OTP in response (no actual SMS sent)
	c.JSON(http.StatusOK, gin.H{
		"message":    "OTP generated successfully (dev mode - no SMS sent)",
		"phone":      phone,
		"expires_at": expiresAt,
		"expires_in": expiresIn,
		"otp":        otp, // Only in development mode
		"mode":       "development",
	})
}

// VerifyOTP handles POST /api/v1/auth/verify-otp
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req VerifyOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body",
		})
		return
	}

	// Validate phone number
	phone, err := h.phoneValidator.Validate(req.Phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_phone",
			Message: err.Error(),
		})
		return
	}

	// Get real client IP and user agent
	clientIP := utils.GetRealIP(c)
	userAgent := utils.GetUserAgent(c)

	// Get current attempts before validation
	remainingBefore, _ := h.otpService.GetRemainingAttempts(phone)

	// Validate OTP
	valid, err := h.otpService.ValidateOTP(phone, req.OTP)
	if err != nil {
		// Log failed verification
		attempts := 3 - remainingBefore + 1 // Calculated attempts made
		h.auditService.LogOTPVerification(nil, phone, false, attempts, clientIP, userAgent, err.Error())

		// Check specific error types
		switch err {
		case services.ErrOTPExpired:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "otp_expired",
				Message: "OTP has expired. Please request a new one.",
				Code:    "OTP_EXPIRED",
			})
		case services.ErrOTPInvalid:
			remaining, _ := h.otpService.GetRemainingAttempts(phone)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "otp_invalid",
				Message: "Invalid OTP code",
				Code:    "OTP_INVALID",
			})
			c.Header("X-Remaining-Attempts", string(rune(remaining)))
		case services.ErrMaxAttemptsExceeded:
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "max_attempts_exceeded",
				Message: "Maximum OTP validation attempts exceeded. Please request a new OTP.",
				Code:    "MAX_ATTEMPTS",
			})
		case services.ErrNoOTPFound:
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "no_otp_found",
				Message: "No OTP found for this phone number. Please request an OTP first.",
				Code:    "NO_OTP",
			})
		case services.ErrOTPAlreadyUsed:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "otp_already_used",
				Message: "This OTP has already been used. Please request a new one.",
				Code:    "OTP_USED",
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "validation_failed",
				Message: "Failed to validate OTP",
			})
		}
		return
	}

	if !valid {
		// Log invalid OTP
		attempts := 3 - remainingBefore + 1
		h.auditService.LogOTPVerification(nil, phone, false, attempts, clientIP, userAgent, "invalid_code")

		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "otp_invalid",
			Message: "Invalid OTP code",
		})
		return
	}

	// Get or create user
	user, isNew, err := h.userRepository.GetOrCreateUser(phone)
	if err != nil {
		log.Printf("ERROR: Failed to get or create user for phone %s: %v", phone, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "user_creation_failed",
			Message: "Failed to get or create user",
		})
		return
	}

	// Generate JWT tokens with user's actual data
	accessToken, err := h.jwtService.GenerateAccessToken(
		user.ID,
		user.Phone,
		user.Roles,
		user.ProfileCompleted,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: "Failed to generate access token",
		})
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID, user.Phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: "Failed to generate refresh token",
		})
		return
	}

	// Store refresh token in database
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	// Get device info from request if provided
	deviceID := c.GetHeader("X-Device-ID")
	deviceType := c.GetHeader("X-Device-Type")

	err = h.refreshTokenRepository.StoreRefreshToken(
		user.ID,
		refreshToken,
		deviceID,
		deviceType,
		clientIP,
		userAgent,
		expiresAt,
	)
	if err != nil {
		// Log error but don't fail the login
		// In production, you'd want proper logging here
	}

	// Log successful OTP verification and login
	h.auditService.LogOTPVerification(&user.ID, phone, true, 3-remainingBefore+1, clientIP, userAgent, "")
	h.auditService.LogLogin(user.ID, phone, clientIP, userAgent, deviceID, deviceType)

	c.JSON(http.StatusOK, VerifyOTPResponse{
		Message:         "OTP verified successfully",
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		ExpiresIn:       3600, // 1 hour
		IsNewUser:       isNew,
		ProfileComplete: user.ProfileCompleted,
		Roles:           user.Roles, // Include user roles in response
	})
}

// VerifyOTPStaff handles POST /api/v1/auth/verify-otp-staff
// This endpoint is specifically for staff app authentication
// It creates users WITHOUT assigning any role initially
func (h *AuthHandler) VerifyOTPStaff(c *gin.Context) {
	var req VerifyOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body",
		})
		return
	}

	// Validate phone number
	phone, err := h.phoneValidator.Validate(req.Phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_phone",
			Message: err.Error(),
		})
		return
	}

	// Get real client IP and user agent
	clientIP := utils.GetRealIP(c)
	userAgent := utils.GetUserAgent(c)

	// Get current attempts before validation
	remainingBefore, _ := h.otpService.GetRemainingAttempts(phone)

	// Validate OTP
	valid, err := h.otpService.ValidateOTP(phone, req.OTP)
	if err != nil {
		// Log failed verification
		attempts := 3 - remainingBefore + 1
		h.auditService.LogOTPVerification(nil, phone, false, attempts, clientIP, userAgent, err.Error())

		// Check specific error types
		switch err {
		case services.ErrOTPExpired:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "otp_expired",
				Message: "OTP has expired. Please request a new one.",
				Code:    "OTP_EXPIRED",
			})
		case services.ErrOTPInvalid:
			remaining, _ := h.otpService.GetRemainingAttempts(phone)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "otp_invalid",
				Message: "Invalid OTP code",
				Code:    "OTP_INVALID",
			})
			c.Header("X-Remaining-Attempts", string(rune(remaining)))
		case services.ErrMaxAttemptsExceeded:
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "max_attempts_exceeded",
				Message: "Maximum OTP validation attempts exceeded. Please request a new OTP.",
				Code:    "MAX_ATTEMPTS",
			})
		case services.ErrNoOTPFound:
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "no_otp_found",
				Message: "No OTP found for this phone number. Please request an OTP first.",
				Code:    "NO_OTP",
			})
		case services.ErrOTPAlreadyUsed:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "otp_already_used",
				Message: "This OTP has already been used. Please request a new one.",
				Code:    "OTP_USED",
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "validation_failed",
				Message: "Failed to validate OTP",
			})
		}
		return
	}

	if !valid {
		// Log invalid OTP
		attempts := 3 - remainingBefore + 1
		h.auditService.LogOTPVerification(nil, phone, false, attempts, clientIP, userAgent, "invalid_code")

		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "otp_invalid",
			Message: "Invalid OTP code",
		})
		return
	}

	// Check if user already exists
	existingUser, err := h.userRepository.GetUserByPhone(phone)
	if err != nil {
		log.Printf("ERROR: Failed to check existing user for phone %s: %v", phone, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "user_check_failed",
			Message: "Failed to check user status",
		})
		return
	}

	var user *models.User
	isNew := false

	if existingUser != nil {
		// EXISTING USER - Validate they can use staff app
		user = existingUser

		// Check if user is a passenger - passengers cannot use staff app
		if h.userRepository.HasRole(user, "passenger") {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "invalid_user_type",
				Message: "You are registered as a passenger. Please use the Passenger app.",
				Code:    "PASSENGER_USER",
			})
			return
		}

		// User already exists and is either staff or has no role - allow login
		log.Printf("INFO: Existing staff user logged in: %s (roles: %v)", phone, user.Roles)
	} else {
		// NEW USER - Create without role
		user, err = h.userRepository.CreateUserWithoutRole(phone)
		if err != nil {
			log.Printf("ERROR: Failed to create staff user for phone %s: %v", phone, err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "user_creation_failed",
				Message: "Failed to create user account",
			})
			return
		}
		isNew = true
		log.Printf("INFO: New staff user created: %s (no role assigned yet)", phone)
	}

	// Generate JWT tokens with user's actual data
	accessToken, err := h.jwtService.GenerateAccessToken(
		user.ID,
		user.Phone,
		user.Roles,
		user.ProfileCompleted,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: "Failed to generate access token",
		})
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID, user.Phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: "Failed to generate refresh token",
		})
		return
	}

	// Store refresh token in database
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	// Get device info from request if provided
	deviceID := c.GetHeader("X-Device-ID")
	deviceType := c.GetHeader("X-Device-Type")

	err = h.refreshTokenRepository.StoreRefreshToken(
		user.ID,
		refreshToken,
		deviceID,
		deviceType,
		clientIP,
		userAgent,
		expiresAt,
	)
	if err != nil {
		// Log error but don't fail the login
		log.Printf("WARNING: Failed to store refresh token for user %s: %v", user.ID, err)
	}

	// Log successful OTP verification and login (staff app)
	h.auditService.LogOTPVerification(&user.ID, phone, true, 3-remainingBefore+1, clientIP, userAgent, "")
	h.auditService.LogLogin(user.ID, phone, clientIP, userAgent, deviceID, deviceType)

	c.JSON(http.StatusOK, VerifyOTPResponse{
		Message:         "OTP verified successfully",
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		ExpiresIn:       3600, // 1 hour
		IsNewUser:       isNew,
		ProfileComplete: user.ProfileCompleted,
		Roles:           user.Roles, // Include user roles - empty [] for new users, ["driver"]/["conductor"] for existing staff
	})
}

// GetOTPStatus handles GET /api/v1/auth/otp-status/:phone
func (h *AuthHandler) GetOTPStatus(c *gin.Context) {
	phone := c.Param("phone")

	// Validate phone number
	phone, err := h.phoneValidator.Validate(phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_phone",
			Message: err.Error(),
		})
		return
	}

	// Get OTP stats
	stats, err := h.otpService.GetOTPStats(phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "stats_retrieval_failed",
			Message: "Failed to retrieve OTP status",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ProfileResponse represents the user profile data
type ProfileResponse struct {
	ID               string   `json:"id"`
	Phone            string   `json:"phone"`
	Email            *string  `json:"email"`
	FirstName        *string  `json:"first_name"`
	LastName         *string  `json:"last_name"`
	NIC              *string  `json:"nic"`
	DateOfBirth      *string  `json:"date_of_birth"`
	Address          *string  `json:"address"`
	City             *string  `json:"city"`
	PostalCode       *string  `json:"postal_code"`
	Roles            []string `json:"roles"`
	ProfilePhotoURL  *string  `json:"profile_photo_url"`
	ProfileCompleted bool     `json:"profile_completed"`
	Status           string   `json:"status"`
	PhoneVerified    bool     `json:"phone_verified"`
	EmailVerified    bool     `json:"email_verified"`
}

// UpdateProfileRequest represents the request to update profile
type UpdateProfileRequest struct {
	FirstName  string `json:"first_name" binding:"required"`
	LastName   string `json:"last_name" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Address    string `json:"address" binding:"required"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
}

// GetProfile handles GET /api/v1/auth/profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get user from database
	user, err := h.userRepository.GetUserByID(userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "profile_retrieval_failed",
			Message: "Failed to retrieve user profile",
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User profile not found",
		})
		return
	}

	// Convert to response format
	response := ProfileResponse{
		ID:               user.ID.String(),
		Phone:            user.Phone,
		Roles:            user.Roles,
		ProfileCompleted: user.ProfileCompleted,
		Status:           user.Status,
		PhoneVerified:    user.PhoneVerified,
		EmailVerified:    user.EmailVerified,
	}

	// Handle nullable fields
	if user.Email.Valid {
		response.Email = &user.Email.String
	}
	if user.FirstName.Valid {
		response.FirstName = &user.FirstName.String
	}
	if user.LastName.Valid {
		response.LastName = &user.LastName.String
	}
	if user.NIC.Valid {
		response.NIC = &user.NIC.String
	}
	if user.DateOfBirth.Valid {
		dob := user.DateOfBirth.Time.Format("2006-01-02")
		response.DateOfBirth = &dob
	}
	if user.Address.Valid {
		response.Address = &user.Address.String
	}
	if user.City.Valid {
		response.City = &user.City.String
	}
	if user.PostalCode.Valid {
		response.PostalCode = &user.PostalCode.String
	}
	if user.ProfilePhotoURL.Valid {
		response.ProfilePhotoURL = &user.ProfilePhotoURL.String
	}

	c.JSON(http.StatusOK, response)
}

// UpdateProfile handles PUT /api/v1/auth/profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse request body
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Update profile
	err := h.userRepository.UpdateProfile(
		userCtx.UserID,
		req.FirstName,
		req.LastName,
		req.Email,
		req.Address,
		req.City,
		req.PostalCode,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "profile_update_failed",
			Message: "Failed to update profile",
		})
		return
	}

	// Update profile completion status
	err = h.userRepository.UpdateProfileCompletion(userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "profile_completion_check_failed",
			Message: "Failed to check profile completion",
		})
		return
	}

	// Get updated user profile
	user, err := h.userRepository.GetUserByID(userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "profile_retrieval_failed",
			Message: "Failed to retrieve updated profile",
		})
		return
	}

	// Convert to response format
	response := ProfileResponse{
		ID:               user.ID.String(),
		Phone:            user.Phone,
		Roles:            user.Roles,
		ProfileCompleted: user.ProfileCompleted,
		Status:           user.Status,
		PhoneVerified:    user.PhoneVerified,
		EmailVerified:    user.EmailVerified,
	}

	// Handle nullable fields
	if user.Email.Valid {
		response.Email = &user.Email.String
	}
	if user.FirstName.Valid {
		response.FirstName = &user.FirstName.String
	}
	if user.LastName.Valid {
		response.LastName = &user.LastName.String
	}
	if user.NIC.Valid {
		response.NIC = &user.NIC.String
	}
	if user.DateOfBirth.Valid {
		dob := user.DateOfBirth.Time.Format("2006-01-02")
		response.DateOfBirth = &dob
	}
	if user.Address.Valid {
		response.Address = &user.Address.String
	}
	if user.City.Valid {
		response.City = &user.City.String
	}
	if user.PostalCode.Valid {
		response.PostalCode = &user.PostalCode.String
	}
	if user.ProfilePhotoURL.Valid {
		response.ProfilePhotoURL = &user.ProfilePhotoURL.String
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"profile": response,
	})
}

// RefreshTokenRequest represents the request to refresh access token
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	DeviceID     string `json:"device_id"`
	DeviceType   string `json:"device_type"`
}

// RefreshTokenResponse represents the response after refreshing token
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in_seconds"`
	TokenType    string `json:"token_type"`
}

// RefreshToken handles POST /api/v1/auth/refresh-token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request body",
		})
		return
	}

	// Validate refresh token
	claims, err := h.jwtService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "invalid_token",
			Message: "Invalid or expired refresh token",
		})
		return
	}

	// Check if token is revoked in database
	revoked, err := h.refreshTokenRepository.IsTokenRevoked(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_check_failed",
			Message: "Failed to verify token status",
		})
		return
	}

	if revoked {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "token_revoked",
			Message: "Refresh token has been revoked",
		})
		return
	}

	// Get user from database to ensure they still exist and get current profile status
	user, err := h.userRepository.GetUserByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "user_fetch_failed",
			Message: "Failed to fetch user information",
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "user_not_found",
			Message: "User no longer exists",
		})
		return
	}

	// Check if user is active
	if user.Status != "active" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "user_inactive",
			Message: "User account is not active",
		})
		return
	}

	// Update last used timestamp for the old refresh token
	if err := h.refreshTokenRepository.UpdateLastUsed(req.RefreshToken); err != nil {
		// Log error but don't fail the request
		// In production, you'd log this properly
	}

	// Revoke the old refresh token (token rotation)
	if err := h.refreshTokenRepository.RevokeToken(req.RefreshToken); err != nil {
		// Log error but continue with new token generation
	}

	// Generate new access token
	accessToken, err := h.jwtService.GenerateAccessToken(
		user.ID,
		user.Phone,
		user.Roles,
		user.ProfileCompleted,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: "Failed to generate new access token",
		})
		return
	}

	// Generate new refresh token
	newRefreshToken, err := h.jwtService.GenerateRefreshToken(user.ID, user.Phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: "Failed to generate new refresh token",
		})
		return
	}

	// Store new refresh token in database
	clientIP := utils.GetRealIP(c)
	userAgent := utils.GetUserAgent(c)
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	err = h.refreshTokenRepository.StoreRefreshToken(
		user.ID,
		newRefreshToken,
		req.DeviceID,
		req.DeviceType,
		clientIP,
		userAgent,
		expiresAt,
	)
	if err != nil {
		// Log failed token refresh
		h.auditService.LogTokenRefresh(user.ID, clientIP, userAgent, false)

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_storage_failed",
			Message: "Failed to store new refresh token",
		})
		return
	}

	// Log successful token refresh
	h.auditService.LogTokenRefresh(user.ID, clientIP, userAgent, true)

	c.JSON(http.StatusOK, RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    3600, // 1 hour
		TokenType:    "Bearer",
	})
}

// LogoutRequest represents the request to logout
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
	LogoutAll    bool   `json:"logout_all"` // If true, logout from all devices
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get real client IP and user agent for audit logging
	clientIP := utils.GetRealIP(c)
	userAgent := utils.GetUserAgent(c)

	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// If no body provided, default to single device logout
		log.Printf("No request body, defaulting to single device logout for user %s", userCtx.UserID)
		req.LogoutAll = false
	}

	// Log the received request for debugging
	log.Printf("Logout request received - User: %s, LogoutAll: %v, HasRefreshToken: %v",
		userCtx.UserID, req.LogoutAll, req.RefreshToken != "")

	if req.LogoutAll {
		// Revoke all refresh tokens for the user
		log.Printf("Revoking all tokens for user %s", userCtx.UserID)
		err := h.refreshTokenRepository.RevokeAllUserTokens(userCtx.UserID)
		if err != nil {
			log.Printf("ERROR: Failed to revoke all tokens for user %s: %v", userCtx.UserID, err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "logout_failed",
				Message: "Failed to logout from all devices",
			})
			return
		}

		// Log logout from all devices
		h.auditService.LogLogout(userCtx.UserID, clientIP, userAgent, true)

		c.JSON(http.StatusOK, gin.H{
			"message": "Successfully logged out from all devices",
		})
		return
	}

	// Single device logout
	// If specific refresh token provided, revoke it
	if req.RefreshToken != "" {
		log.Printf("Revoking specific token for user %s", userCtx.UserID)
		err := h.refreshTokenRepository.RevokeToken(req.RefreshToken)
		if err != nil {
			log.Printf("ERROR: Failed to revoke token for user %s: %v", userCtx.UserID, err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "logout_failed",
				Message: "Failed to revoke token",
			})
			return
		}

		// Log single device logout
		h.auditService.LogLogout(userCtx.UserID, clientIP, userAgent, false)

		c.JSON(http.StatusOK, gin.H{
			"message": "Successfully logged out",
		})
		return
	}

	// If no refresh token provided, revoke the most recent active token
	// This handles the case where Flutter sends logout_all: false but no refresh_token
	log.Printf("No refresh token provided, revoking most recent token for user %s", userCtx.UserID)
	err := h.refreshTokenRepository.RevokeMostRecentToken(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to revoke most recent token for user %s: %v", userCtx.UserID, err)
		// Don't fail the logout - client-side logout is still valid
		log.Printf("WARN: Server-side token revocation failed, but allowing logout")
	}

	// Log logout
	h.auditService.LogLogout(userCtx.UserID, clientIP, userAgent, false)

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully logged out",
	})
}
