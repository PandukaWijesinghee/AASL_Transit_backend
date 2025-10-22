package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/config"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/handlers"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/services"
	"github.com/smarttransit/sms-auth-backend/pkg/jwt"
	"github.com/smarttransit/sms-auth-backend/pkg/sms"
	"github.com/smarttransit/sms-auth-backend/pkg/validator"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	logger.Info("Starting SmartTransit SMS Authentication Backend")
	logger.Infof("Version: %s, Build Time: %s", version, buildTime)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level
	logLevel, err := logrus.ParseLevel(cfg.Server.LogLevel)
	if err != nil {
		logger.Warn("Invalid log level, using INFO")
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	// Set Gin mode
	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Initialize database connection
	logger.Info("Connecting to database...")
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	logger.Info("Database connection established")

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize services
	logger.Info("Initializing services...")
	jwtService := jwt.NewService(
		cfg.JWT.Secret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTokenExpiry,
		cfg.JWT.RefreshTokenExpiry,
	)
	otpService := services.NewOTPService(db)
	phoneValidator := validator.NewPhoneValidator()
	rateLimitService := services.NewRateLimitService(db)
	auditService := services.NewAuditService(db)
	userRepository := database.NewUserRepository(db)
	refreshTokenRepository := database.NewRefreshTokenRepository(db)
	userSessionRepository := database.NewUserSessionRepository(db)

	// Initialize staff-related repositories
	staffRepository := database.NewBusStaffRepository(db)
	ownerRepository := database.NewBusOwnerRepository(db)

	// Initialize staff service
	staffService := services.NewStaffService(staffRepository, ownerRepository, userRepository)

	// Initialize SMS Gateway (Dialog)
	var smsGateway sms.SMSGateway

	// Get both app hashes for SMS auto-read
	driverAppHash := cfg.SMS.DriverAppHash
	passengerAppHash := cfg.SMS.PassengerAppHash

	if driverAppHash != "" || passengerAppHash != "" {
		logger.Info("SMS auto-read enabled:")
		if driverAppHash != "" {
			logger.Info("  Driver app hash: " + driverAppHash)
		}
		if passengerAppHash != "" {
			logger.Info("  Passenger app hash: " + passengerAppHash)
		}
	}

	if cfg.SMS.Mode == "production" {
		logger.Info("Initializing Dialog SMS Gateway in production mode...")

		// Choose gateway based on method
		if cfg.SMS.Method == "url" {
			logger.Info("Using Dialog URL method (GET request with esmsqk)")
			urlGateway := sms.NewDialogURLGateway(cfg.SMS.ESMSQK, cfg.SMS.Mask, driverAppHash, passengerAppHash)
			smsGateway = urlGateway
		} else {
			logger.Info("Using Dialog API v2 method (POST with authentication)")
			apiGateway := sms.NewDialogGateway(sms.DialogConfig{
				APIURL:   cfg.SMS.APIURL,
				Username: cfg.SMS.Username,
				Password: cfg.SMS.Password,
				Mask:     cfg.SMS.Mask,
				AppHash:  passengerAppHash, // Use passenger hash as default
			})
			smsGateway = apiGateway
		}

		logger.Info("Dialog SMS Gateway initialized")
	} else {
		logger.Info("SMS Gateway in development mode (no actual SMS will be sent)")
		// Still initialize but won't be used in dev mode
		smsGateway = sms.NewDialogGateway(sms.DialogConfig{
			APIURL:   cfg.SMS.APIURL,
			Username: cfg.SMS.Username,
			Password: cfg.SMS.Password,
			Mask:     cfg.SMS.Mask,
			AppHash:  passengerAppHash,
		})
	}

	logger.Info("Services initialized")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(
		jwtService,
		otpService,
		phoneValidator,
		rateLimitService,
		auditService,
		userRepository,
		refreshTokenRepository,
		userSessionRepository,
		smsGateway,
		cfg,
	)

	// Initialize staff handler
	staffHandler := handlers.NewStaffHandler(staffService, userRepository, staffRepository)

	// Initialize Gin router
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(requestLogger(logger))

	// CORS configuration
	corsConfig := cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     cfg.CORS.AllowedMethods,
		AllowHeaders:     cfg.CORS.AllowedHeaders,
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	// Health check endpoint
	router.GET("/health", healthCheckHandler(db))

	// Set environment in context for development mode
	router.Use(func(c *gin.Context) {
		c.Set("environment", cfg.Server.Environment)
		c.Next()
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Debug endpoint - shows all request headers and IP detection (public)
		v1.GET("/debug/headers", debugHeadersHandler())

		// Authentication routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/send-otp", authHandler.SendOTP)
			auth.POST("/verify-otp", authHandler.VerifyOTP)
			auth.POST("/verify-otp-staff", authHandler.VerifyOTPStaff) // New staff-specific endpoint
			auth.GET("/otp-status/:phone", authHandler.GetOTPStatus)
			auth.POST("/refresh-token", authHandler.RefreshToken)
			auth.POST("/refresh", authHandler.RefreshToken) // Alias for mobile compatibility

			// Protected routes (require JWT authentication)
			protected := auth.Group("")
			protected.Use(middleware.AuthMiddleware(jwtService))
			{
				protected.POST("/logout", authHandler.Logout)
			}
		}

		// User routes (protected)
		user := v1.Group("/user")
		user.Use(middleware.AuthMiddleware(jwtService))
		{
			user.GET("/profile", authHandler.GetProfile)
			user.PUT("/profile", authHandler.UpdateProfile)
		}

		// Staff routes
		staff := v1.Group("/staff")
		{
			// Public routes (no authentication required)
			staff.POST("/check-registration", staffHandler.CheckRegistration)
			staff.POST("/register", staffHandler.RegisterStaff)
			staff.GET("/bus-owners/search", staffHandler.SearchBusOwners)

			// Protected routes (require JWT authentication)
			staffProtected := staff.Group("")
			staffProtected.Use(middleware.AuthMiddleware(jwtService))
			{
				staffProtected.GET("/profile", staffHandler.GetProfile)
				staffProtected.PUT("/profile", staffHandler.UpdateProfile)
			}
		}
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Infof("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited successfully")
}

// requestLogger middleware for logging HTTP requests
func requestLogger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Log incoming request
		logger.WithFields(logrus.Fields{
			"method":     c.Request.Method,
			"path":       path,
			"query":      query,
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}).Info("Incoming request")

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		// Build log entry with basic fields
		fields := logrus.Fields{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       path,
			"query":      query,
			"ip":         c.ClientIP(),
			"latency_ms": latency.Milliseconds(),
			"user_agent": c.Request.UserAgent(),
		}

		// Add authorization header presence (not the actual token for security)
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			fields["has_auth"] = true
			if len(authHeader) > 20 {
				fields["auth_type"] = authHeader[:20] + "..." // Show Bearer prefix only
			}
		} else {
			fields["has_auth"] = false
		}

		// Add user context if available
		if userID, exists := c.Get("user_id"); exists {
			fields["user_id"] = userID
		}
		if phone, exists := c.Get("phone"); exists {
			fields["phone"] = phone
		}
		if roles, exists := c.Get("roles"); exists {
			fields["roles"] = roles
		}

		entry := logger.WithFields(fields)

		// Log errors with more details
		if len(c.Errors) > 0 {
			// Add error details
			for i, err := range c.Errors {
				entry = entry.WithField(fmt.Sprintf("error_%d", i), err.Error())
				if err.Meta != nil {
					entry = entry.WithField(fmt.Sprintf("error_%d_meta", i), err.Meta)
				}
			}
			entry.Error("Request failed with errors")
		} else {
			// Log based on status code
			status := c.Writer.Status()
			if status >= 500 {
				entry.Error("Request completed with server error")
			} else if status >= 400 {
				entry.Warn("Request completed with client error")
			} else {
				entry.Info("Request completed successfully")
			}
		}
	}
}

// healthCheckHandler returns a health check endpoint
func healthCheckHandler(db database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check database connection
		dbStatus := "healthy"
		if err := db.Ping(); err != nil {
			dbStatus = "unhealthy"
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "unhealthy",
				"database": dbStatus,
				"error":    err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"database":  dbStatus,
			"version":   version,
			"timestamp": time.Now().Unix(),
		})
	}
}

// debugHeadersHandler shows all request headers for debugging IP issues
func debugHeadersHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Collect all headers
		headers := make(map[string]string)
		for name, values := range c.Request.Header {
			headers[name] = values[0] // Take first value
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Debug information for IP detection",
			"headers": headers,
			"ip_detection": gin.H{
				"gin_clientip":     c.ClientIP(),
				"remote_addr":      c.Request.RemoteAddr,
				"x_real_ip":        c.Request.Header.Get("X-Real-IP"),
				"x_forwarded_for":  c.Request.Header.Get("X-Forwarded-For"),
				"x_forwarded_host": c.Request.Header.Get("X-Forwarded-Host"),
				"x_forwarded_proto": c.Request.Header.Get("X-Forwarded-Proto"),
			},
			"user_agent": c.Request.UserAgent(),
			"timestamp":  time.Now().Unix(),
		})
	}
}
