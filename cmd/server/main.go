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
	logger.Info("🔍 DEBUG: Lounge Owner registration system ENABLED")
	logger.Info("🔍 DEBUG: This build includes lounge owner routes")

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
	permitRepository := database.NewRoutePermitRepository(db)
	busRepository := database.NewBusRepository(db)

	// Initialize lounge owner repositories
	// Type assertion needed: db is interface DB, but repositories need *sqlx.DB
	sqlxDB, ok := db.(*database.PostgresDB)
	if !ok {
		logger.Fatal("Failed to cast database connection to PostgresDB")
	}
	loungeOwnerRepository := database.NewLoungeOwnerRepository(sqlxDB.DB)
	loungeRepository := database.NewLoungeRepository(sqlxDB.DB)
	loungeStaffRepository := database.NewLoungeStaffRepository(sqlxDB.DB)

	// Initialize staff service
	staffService := services.NewStaffService(staffRepository, ownerRepository, userRepository)

	// Initialize trip scheduling repositories
	tripScheduleRepo := database.NewTripScheduleRepository(sqlxDB.DB)
	scheduledTripRepo := database.NewScheduledTripRepository(sqlxDB.DB)
	masterRouteRepo := database.NewMasterRouteRepository(sqlxDB.DB)
	systemSettingRepo := database.NewSystemSettingRepository(sqlxDB.DB)

	// Initialize trip generator service
	tripGeneratorSvc := services.NewTripGeneratorService(
		tripScheduleRepo,
		scheduledTripRepo,
		busRepository,
		systemSettingRepo,
	)

	// Initialize and start cron service
	cronService := services.NewCronService(tripGeneratorSvc)
	if err := cronService.Start(); err != nil {
		logger.Fatalf("Failed to start cron service: %v", err)
	}
	logger.Info("✓ Cron service started - Trip auto-generation enabled")

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

	// Initialize bus owner and permit handlers
	busOwnerHandler := handlers.NewBusOwnerHandler(ownerRepository, permitRepository, userRepository, staffRepository)
	permitHandler := handlers.NewPermitHandler(permitRepository, ownerRepository, masterRouteRepo)
	busHandler := handlers.NewBusHandler(busRepository, permitRepository, ownerRepository)
	masterRouteHandler := handlers.NewMasterRouteHandler(masterRouteRepo)

	// Initialize bus owner route repository and handler
	busOwnerRouteRepo := database.NewBusOwnerRouteRepository(db)
	busOwnerRouteHandler := handlers.NewBusOwnerRouteHandler(busOwnerRouteRepo, ownerRepository)

	// Initialize lounge owner, lounge, staff, and admin handlers
	logger.Info("🔍 DEBUG: Initializing lounge handlers...")
	loungeOwnerHandler := handlers.NewLoungeOwnerHandler(loungeOwnerRepository, userRepository)
	loungeHandler := handlers.NewLoungeHandler(loungeRepository, loungeOwnerRepository)
	loungeStaffHandler := handlers.NewLoungeStaffHandler(loungeStaffRepository, loungeRepository, loungeOwnerRepository)
	logger.Info("🔍 DEBUG: Lounge handlers initialized successfully")
	adminHandler := handlers.NewAdminHandler(loungeOwnerRepository, loungeRepository, userRepository)

	// Initialize trip scheduling handlers
	tripScheduleHandler := handlers.NewTripScheduleHandler(
		tripScheduleRepo,
		permitRepository,
		ownerRepository,
		busRepository,
		busOwnerRouteRepo,
		tripGeneratorSvc,
	)

	scheduledTripHandler := handlers.NewScheduledTripHandler(
		scheduledTripRepo,
		tripScheduleRepo,
		permitRepository,
		ownerRepository,
		busOwnerRouteRepo,
		busRepository,
		systemSettingRepo,
	)
	systemSettingHandler := handlers.NewSystemSettingHandler(systemSettingRepo)
	logger.Info("Trip scheduling handlers initialized")

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

		// Debug endpoint - list all registered routes
		v1.GET("/debug/routes", func(c *gin.Context) {
			routes := router.Routes()
			routeList := make([]map[string]string, 0)
			for _, route := range routes {
				routeList = append(routeList, map[string]string{
					"method": route.Method,
					"path":   route.Path,
				})
			}
			c.JSON(200, gin.H{
				"message":      "Registered routes",
				"total_routes": len(routeList),
				"routes":       routeList,
			})
		})

		// Authentication routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/send-otp", authHandler.SendOTP)
			auth.POST("/verify-otp", authHandler.VerifyOTP)
			auth.POST("/verify-otp-staff", authHandler.VerifyOTPStaff) // Staff-specific endpoint
			auth.POST("/verify-otp-lounge-owner", func(c *gin.Context) {
				authHandler.VerifyOTPLoungeOwner(c, loungeOwnerRepository)
			}) // Lounge owner-specific endpoint
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

		// Bus Owner routes (all protected)
		busOwner := v1.Group("/bus-owner")
		busOwner.Use(middleware.AuthMiddleware(jwtService))
		{
			busOwner.GET("/profile", busOwnerHandler.GetProfile)
			busOwner.GET("/profile-status", busOwnerHandler.CheckProfileStatus)
			busOwner.POST("/complete-onboarding", busOwnerHandler.CompleteOnboarding)
			busOwner.GET("/staff", busOwnerHandler.GetStaff)  // Get all staff (drivers & conductors)
			busOwner.POST("/staff", busOwnerHandler.AddStaff) // Add driver or conductor
		}

		// Bus Owner Routes (custom route configurations)
		busOwnerRoutes := v1.Group("/bus-owner-routes")
		busOwnerRoutes.Use(middleware.AuthMiddleware(jwtService))
		{
			busOwnerRoutes.POST("", busOwnerRouteHandler.CreateRoute)
			busOwnerRoutes.GET("", busOwnerRouteHandler.GetRoutes)
			busOwnerRoutes.GET("/:id", busOwnerRouteHandler.GetRouteByID)
			busOwnerRoutes.GET("/by-master-route/:master_route_id", busOwnerRouteHandler.GetRoutesByMasterRoute)
			busOwnerRoutes.PUT("/:id", busOwnerRouteHandler.UpdateRoute)
			busOwnerRoutes.DELETE("/:id", busOwnerRouteHandler.DeleteRoute)
		}

		// Lounge Owner routes (all protected)
		logger.Info("🏢 Registering Lounge Owner routes...")
		loungeOwner := v1.Group("/lounge-owner")
		loungeOwner.Use(middleware.AuthMiddleware(jwtService))
		{
			// Registration endpoints
			logger.Info("  ✅ POST /api/v1/lounge-owner/register/business-info")
			loungeOwner.POST("/register/business-info", loungeOwnerHandler.SaveBusinessAndManagerInfo)
			logger.Info("  ✅ POST /api/v1/lounge-owner/register/upload-manager-nic")
			loungeOwner.POST("/register/upload-manager-nic", loungeOwnerHandler.UploadManagerNIC)
			logger.Info("  ✅ POST /api/v1/lounge-owner/register/add-lounge")
			loungeOwner.POST("/register/add-lounge", loungeHandler.AddLounge)
			logger.Info("  ✅ GET /api/v1/lounge-owner/registration/progress")
			loungeOwner.GET("/registration/progress", loungeOwnerHandler.GetRegistrationProgress)

			// Profile endpoints
			logger.Info("  ✅ GET /api/v1/lounge-owner/profile")
			loungeOwner.GET("/profile", loungeOwnerHandler.GetProfile)
		}
		logger.Info("🏢 Lounge Owner routes registered successfully")

		// Lounge routes (protected)
		logger.Info("🏨 Registering Lounge routes...")
		lounges := v1.Group("/lounges")
		{
			// Public routes (no authentication)
			logger.Info("  ✅ GET /api/v1/lounges/city/:city (public)")
			lounges.GET("/city/:city", loungeHandler.GetLoungesByCity)

			// Protected routes (require JWT authentication)
			loungesProtected := lounges.Group("")
			loungesProtected.Use(middleware.AuthMiddleware(jwtService))
			{
				logger.Info("  ✅ GET /api/v1/lounges/my-lounges")
				loungesProtected.GET("/my-lounges", loungeHandler.GetMyLounges)
				logger.Info("  ✅ GET /api/v1/lounges/:id")
				loungesProtected.GET("/:id", loungeHandler.GetLoungeByID)
				logger.Info("  ✅ PUT /api/v1/lounges/:id")
				loungesProtected.PUT("/:id", loungeHandler.UpdateLounge)
				logger.Info("  ✅ DELETE /api/v1/lounges/:id")
				loungesProtected.DELETE("/:id", loungeHandler.DeleteLounge)

				// Staff management for specific lounge (use :id to match other lounge routes)
				logger.Info("  ✅ POST /api/v1/lounges/:id/staff")
				loungesProtected.POST("/:id/staff", loungeStaffHandler.AddStaff)
				logger.Info("  ✅ GET /api/v1/lounges/:id/staff")
				loungesProtected.GET("/:id/staff", loungeStaffHandler.GetStaffByLounge)
				logger.Info("  ✅ PUT /api/v1/lounges/:id/staff/:staff_id/permission")
				loungesProtected.PUT("/:id/staff/:staff_id/permission", loungeStaffHandler.UpdateStaffPermission)
				logger.Info("  ✅ PUT /api/v1/lounges/:id/staff/:staff_id/status")
				loungesProtected.PUT("/:id/staff/:staff_id/status", loungeStaffHandler.UpdateStaffStatus)
				logger.Info("  ✅ DELETE /api/v1/lounges/:id/staff/:staff_id")
				loungesProtected.DELETE("/:id/staff/:staff_id", loungeStaffHandler.RemoveStaff)
			}
		}
		logger.Info("� Lounge routes registered successfully")

		// Staff profile routes (for lounge staff members)
		logger.Info("👤 Registering Staff profile routes...")
		staffProfile := v1.Group("/staff")
		staffProfile.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ GET /api/v1/staff/my-profile")
			staffProfile.GET("/my-profile", loungeStaffHandler.GetMyStaffProfile)
		}
		logger.Info("👤 Staff profile routes registered successfully")

		// Permit routes (all protected)
		permits := v1.Group("/permits")
		permits.Use(middleware.AuthMiddleware(jwtService))
		{
			permits.GET("", permitHandler.GetAllPermits)
			permits.GET("/valid", permitHandler.GetValidPermits)
			permits.GET("/:id", permitHandler.GetPermitByID)
			permits.GET("/:id/route-details", permitHandler.GetRouteDetails)
			permits.POST("", permitHandler.CreatePermit)
			permits.PUT("/:id", permitHandler.UpdatePermit)
			permits.DELETE("/:id", permitHandler.DeletePermit)
		}

		// Master Routes (all protected - for dropdown selection)
		masterRoutes := v1.Group("/master-routes")
		masterRoutes.Use(middleware.AuthMiddleware(jwtService))
		{
			masterRoutes.GET("", masterRouteHandler.ListMasterRoutes)
			masterRoutes.GET("/:id", masterRouteHandler.GetMasterRouteByID)
		}

		// Bus routes (all protected)
		buses := v1.Group("/buses")
		buses.Use(middleware.AuthMiddleware(jwtService))
		{
			buses.GET("", busHandler.GetAllBuses)
			buses.GET("/:id", busHandler.GetBusByID)
			buses.POST("", busHandler.CreateBus)
			buses.PUT("/:id", busHandler.UpdateBus)
			buses.DELETE("/:id", busHandler.DeleteBus)
			buses.GET("/status/:status", busHandler.GetBusesByStatus)
		}

		// Trip Schedule routes (all protected - bus owners only)
		tripSchedules := v1.Group("/trip-schedules")
		tripSchedules.Use(middleware.AuthMiddleware(jwtService))
		{
			tripSchedules.GET("", tripScheduleHandler.GetAllSchedules)
			tripSchedules.POST("", tripScheduleHandler.CreateSchedule)
			tripSchedules.GET("/:id", tripScheduleHandler.GetScheduleByID)
			tripSchedules.PUT("/:id", tripScheduleHandler.UpdateSchedule)
			tripSchedules.DELETE("/:id", tripScheduleHandler.DeleteSchedule)
			tripSchedules.POST("/:id/deactivate", tripScheduleHandler.DeactivateSchedule)
		}

		// Timetable routes (new timetable system - all protected)
		timetables := v1.Group("/timetables")
		timetables.Use(middleware.AuthMiddleware(jwtService))
		{
			timetables.POST("", tripScheduleHandler.CreateTimetable)
		}

		// Special Trip routes (one-time trips, not from timetable - all protected)
		specialTrips := v1.Group("/special-trips")
		specialTrips.Use(middleware.AuthMiddleware(jwtService))
		{
			specialTrips.POST("", scheduledTripHandler.CreateSpecialTrip)
		}

		// Scheduled Trip routes (all protected - bus owners only)
		scheduledTrips := v1.Group("/scheduled-trips")
		scheduledTrips.Use(middleware.AuthMiddleware(jwtService))
		{
			scheduledTrips.GET("", scheduledTripHandler.GetTripsByDateRange)
			scheduledTrips.GET("/:id", scheduledTripHandler.GetTripByID)
			scheduledTrips.PATCH("/:id", scheduledTripHandler.UpdateTrip)
			scheduledTrips.POST("/:id/cancel", scheduledTripHandler.CancelTrip)
		}

		// Permit-specific trip routes
		permits.GET("/:id/trip-schedules", tripScheduleHandler.GetSchedulesByPermit)
		permits.GET("/:id/scheduled-trips", scheduledTripHandler.GetTripsByPermit)

		// Public bookable trips (no auth required)
		v1.GET("/bookable-trips", scheduledTripHandler.GetBookableTrips)

		// System Settings routes (protected)
		systemSettings := v1.Group("/system-settings")
		systemSettings.Use(middleware.AuthMiddleware(jwtService))
		{
			systemSettings.GET("", systemSettingHandler.GetAllSettings)
			systemSettings.GET("/:key", systemSettingHandler.GetSettingByKey)
			systemSettings.PUT("/:key", systemSettingHandler.UpdateSetting)
		}

		// Admin cron management routes (optional - for testing)
		admin := v1.Group("/admin")
		// TODO: Add admin auth middleware
		{
			// Cron management
			admin.POST("/cron/generate-trips", func(c *gin.Context) {
				cronService.RunGenerateFutureTripsNow()
				c.JSON(200, gin.H{"message": "Trip generation triggered"})
			})

			admin.POST("/cron/fill-missing", func(c *gin.Context) {
				cronService.RunFillMissingTripsNow()
				c.JSON(200, gin.H{"message": "Fill missing trips triggered"})
			})

			admin.GET("/cron/status", func(c *gin.Context) {
				status := cronService.GetJobStatus()
				c.JSON(200, status)
			})

			// Lounge Owner approval (TODO: Implement)
			admin.GET("/lounge-owners/pending", adminHandler.GetPendingLoungeOwners)
			admin.GET("/lounge-owners/:id", adminHandler.GetLoungeOwnerDetails)
			admin.POST("/lounge-owners/:id/approve", adminHandler.ApproveLoungeOwner)
			admin.POST("/lounge-owners/:id/reject", adminHandler.RejectLoungeOwner)

			// Lounge approval (TODO: Implement)
			admin.GET("/lounges/pending", adminHandler.GetPendingLounges)
			admin.POST("/lounges/:id/approve", adminHandler.ApproveLounge)
			admin.POST("/lounges/:id/reject", adminHandler.RejectLounge)

			// Bus Owner approval (TODO: Implement later)
			admin.GET("/bus-owners/pending", adminHandler.GetPendingBusOwners)
			admin.POST("/bus-owners/:id/approve", adminHandler.ApproveBusOwner)

			// Staff approval (TODO: Implement later)
			admin.GET("/staff/pending", adminHandler.GetPendingStaff)
			admin.POST("/staff/:id/approve", adminHandler.ApproveStaff)

			// Dashboard stats (TODO: Implement)
			admin.GET("/dashboard/stats", adminHandler.GetDashboardStats)
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

	// Stop cron service
	logger.Info("Stopping cron service...")
	cronService.Stop()

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
				"gin_clientip":      c.ClientIP(),
				"remote_addr":       c.Request.RemoteAddr,
				"x_real_ip":         c.Request.Header.Get("X-Real-IP"),
				"x_forwarded_for":   c.Request.Header.Get("X-Forwarded-For"),
				"x_forwarded_host":  c.Request.Header.Get("X-Forwarded-Host"),
				"x_forwarded_proto": c.Request.Header.Get("X-Forwarded-Proto"),
			},
			"user_agent": c.Request.UserAgent(),
			"timestamp":  time.Now().Unix(),
		})
	}
}
