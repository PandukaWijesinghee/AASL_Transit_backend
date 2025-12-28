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

	// Initialize passenger repository
	passengerRepository := database.NewPassengerRepository(db)

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
	seatLayoutRepository := database.NewBusSeatLayoutRepository(sqlxDB.DB)

	// Initialize staff service
	staffService := services.NewStaffService(staffRepository, ownerRepository, userRepository)

	// NOTE: Active trip service is initialized after repositories are ready (see below)

	// Initialize trip scheduling repositories
	tripScheduleRepo := database.NewTripScheduleRepository(sqlxDB.DB)
	scheduledTripRepo := database.NewScheduledTripRepository(sqlxDB.DB)
	masterRouteRepo := database.NewMasterRouteRepository(sqlxDB.DB)
	systemSettingRepo := database.NewSystemSettingRepository(sqlxDB.DB)

	// Initialize active trip repository (for real-time trip tracking)
	activeTripRepo := database.NewActiveTripRepository(db)

	// Initialize trip generator service
	tripGeneratorSvc := services.NewTripGeneratorService(
		tripScheduleRepo,
		scheduledTripRepo,
		busRepository,
		seatLayoutRepository,
		systemSettingRepo,
	)

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
				APIURL:           cfg.SMS.APIURL,
				Username:         cfg.SMS.Username,
				Password:         cfg.SMS.Password,
				Mask:             cfg.SMS.Mask,
				DriverAppHash:    driverAppHash,
				PassengerAppHash: passengerAppHash,
			})
			smsGateway = apiGateway
		}

		logger.Info("Dialog SMS Gateway initialized")
	} else {
		logger.Info("SMS Gateway in development mode (no actual SMS will be sent)")
		// Still initialize but won't be used in dev mode
		smsGateway = sms.NewDialogGateway(sms.DialogConfig{
			APIURL:           cfg.SMS.APIURL,
			Username:         cfg.SMS.Username,
			Password:         cfg.SMS.Password,
			Mask:             cfg.SMS.Mask,
			DriverAppHash:    driverAppHash,
			PassengerAppHash: passengerAppHash,
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
		passengerRepository,
		refreshTokenRepository,
		userSessionRepository,
		smsGateway,
		cfg,
	)

	// Initialize staff handler
	staffHandler := handlers.NewStaffHandler(staffService, userRepository, staffRepository, scheduledTripRepo)

	// Initialize app booking repository (needed for active trip service)
	appBookingRepo := database.NewAppBookingRepository(sqlxDB.DB)

	// Initialize active trip service and handler (for Start Trip / End Trip / Location tracking)
	logger.Info("🚌 Initializing Active Trip tracking system...")
	activeTripService := services.NewActiveTripService(
		activeTripRepo,
		scheduledTripRepo,
		staffRepository,
		busRepository,
		permitRepository,
		appBookingRepo,
	)
	activeTripHandler := handlers.NewActiveTripHandler(activeTripService, staffRepository)
	logger.Info("✓ Active Trip tracking system initialized")

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
	loungeRouteRepository := database.NewLoungeRouteRepository(sqlxDB.DB)
	loungeHandler := handlers.NewLoungeHandler(loungeRepository, loungeOwnerRepository, loungeRouteRepository)
	loungeStaffHandler := handlers.NewLoungeStaffHandler(loungeStaffRepository, loungeRepository, loungeOwnerRepository)

	// Initialize lounge booking system
	logger.Info("🏨 Initializing lounge booking system...")
	loungeBookingRepo := database.NewLoungeBookingRepository(sqlxDB.DB)
	loungeBookingHandler := handlers.NewLoungeBookingHandler(loungeBookingRepo, loungeRepository, loungeOwnerRepository)
	logger.Info("✓ Lounge booking system initialized")

	logger.Info("🔍 DEBUG: Lounge handlers initialized successfully")
	adminHandler := handlers.NewAdminHandler(loungeOwnerRepository, loungeRepository, userRepository)

	// Initialize admin authentication repository, service, and handler
	logger.Info("Initializing admin authentication system...")
	adminUserRepository := database.NewAdminUserRepository(db)
	adminRefreshTokenRepository := database.NewAdminRefreshTokenRepository(db)
	adminAuthService := services.NewAdminAuthService(
		adminUserRepository,
		adminRefreshTokenRepository,
		jwtService,
		cfg.JWT.AccessTokenExpiry,
		cfg.JWT.RefreshTokenExpiry,
	)
	adminAuthHandler := handlers.NewAdminAuthHandler(adminAuthService, logger)
	logger.Info("✓ Admin authentication system initialized")

	// Initialize bus seat layout system
	logger.Info("Initializing bus seat layout system...")
	busSeatLayoutRepository := database.NewBusSeatLayoutRepository(db)
	busSeatLayoutService := services.NewBusSeatLayoutService(busSeatLayoutRepository)
	busSeatLayoutHandler := handlers.NewBusSeatLayoutHandler(busSeatLayoutService, logger)
	logger.Info("✓ Bus seat layout system initialized")

	// Initialize trip scheduling handlers
	tripScheduleHandler := handlers.NewTripScheduleHandler(
		tripScheduleRepo,
		permitRepository,
		ownerRepository,
		busRepository,
		busOwnerRouteRepo,
		tripGeneratorSvc,
	)

	// Initialize Trip Seat and Manual Booking system
	logger.Info("Initializing trip seat and manual booking system...")
	tripSeatRepo := database.NewTripSeatRepository(sqlxDB.DB)
	manualBookingRepo := database.NewManualBookingRepository(sqlxDB.DB)
	logger.Info("✓ Trip seat and manual booking repositories initialized")

	scheduledTripHandler := handlers.NewScheduledTripHandler(
		scheduledTripRepo,
		tripScheduleRepo,
		permitRepository,
		ownerRepository,
		busOwnerRouteRepo,
		busRepository,
		staffRepository,
		systemSettingRepo,
		tripSeatRepo,
	)
	systemSettingHandler := handlers.NewSystemSettingHandler(systemSettingRepo)
	logger.Info("Trip scheduling handlers initialized")

	// Initialize search system
	logger.Info("Initializing search system...")
	searchRepo := database.NewSearchRepository(db)
	searchService := services.NewSearchService(searchRepo, logger)
	searchHandler := handlers.NewSearchHandler(searchService, logger)
	logger.Info("✓ Search system initialized")

	// Initialize Trip Seat Handler (tripSeatRepo already initialized above)
	tripSeatHandler := handlers.NewTripSeatHandler(
		tripSeatRepo,
		manualBookingRepo,
		scheduledTripRepo,
		ownerRepository,
		busOwnerRouteRepo,
	)
	logger.Info("✓ Trip seat handler initialized")

	// Initialize App Booking handler (repository already initialized above for active trip service)
	logger.Info("Initializing app booking handler...")
	appBookingHandler := handlers.NewAppBookingHandler(
		appBookingRepo,
		scheduledTripRepo,
		tripSeatRepo,
		busOwnerRouteRepo,
		logger,
	)
	staffBookingHandler := handlers.NewStaffBookingHandler(appBookingRepo)
	logger.Info("✓ App booking handler initialized")

	// ============================================================================
	// BOOKING ORCHESTRATION SYSTEM (Intent → Payment → Confirm)
	// ============================================================================
	logger.Info("🎯 Initializing Booking Orchestration system...")
	bookingIntentRepo := database.NewBookingIntentRepository(sqlxDB.DB)
	bookingOrchestratorConfig := services.DefaultOrchestratorConfig()

	// Initialize PAYable payment service
	payableService := services.NewPAYableService(&cfg.Payment, logger)
	if payableService.IsConfigured() {
		logger.WithField("environment", payableService.GetEnvironment()).Info("✓ PAYable payment gateway configured")
	} else {
		logger.Warn("⚠️ PAYable payment gateway not configured - using placeholder mode")
	}

	// Initialize payment audit repository for logging all payment events
	paymentAuditRepo := database.NewPaymentAuditRepository(sqlxDB.DB, logger)
	logger.Info("✓ Payment audit repository initialized")

	bookingOrchestratorService := services.NewBookingOrchestratorService(
		bookingIntentRepo,
		tripSeatRepo,
		scheduledTripRepo,
		appBookingRepo,
		loungeBookingRepo,
		loungeRepository,
		busOwnerRouteRepo,
		payableService,
		bookingOrchestratorConfig,
		logger,
	)
	bookingOrchestratorHandler := handlers.NewBookingOrchestratorHandler(
		bookingOrchestratorService,
		payableService,
		paymentAuditRepo,
		logger,
	)
	logger.Info("✓ Booking Orchestration system initialized")

	// Start background job for intent expiration
	intentExpirationService := services.NewIntentExpirationService(bookingIntentRepo, logger)
	intentExpirationService.Start()
	defer intentExpirationService.Stop()

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
		// ============================================================================
		// ACTIVE TRIP PASSENGER MANAGEMENT ROUTES
		// ============================================================================
		logger.Info("👥 Registering Active Trip Passenger Management routes...")
		activeTripPassengers := v1.Group("/active-trips/passengers")
		activeTripPassengers.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ GET /api/v1/active-trips/passengers - Get passengers for active trip")
			activeTripPassengers.GET("", activeTripHandler.GetPassengers)

			logger.Info("  ✅ PUT /api/v1/active-trips/passengers/board - Board passenger")
			activeTripPassengers.PUT("/board", activeTripHandler.BoardPassenger)

			logger.Info("  ✅ PUT /api/v1/active-trips/passengers/pending - Set passenger to pending")
			activeTripPassengers.PUT("/pending", activeTripHandler.UpdatePassengerStatusToPending)

			logger.Info("  ✅ PUT /api/v1/active-trips/passengers/cancelled - Cancel passenger")
			activeTripPassengers.PUT("/cancelled", activeTripHandler.CancelPassenger)

			logger.Info("  ✅ POST /api/v1/active-trips/passengers/verify - Verify passenger booking")
			activeTripPassengers.POST("/verify", activeTripHandler.VerifyPassenger)

			logger.Info("  ✅ PUT /api/v1/active-trips/passengers/confirmed - Confirm passenger")
			activeTripPassengers.PUT("/confirmed", activeTripHandler.UpdatePassengerStatusToConfirmed)

			logger.Info("  ✅ PUT /api/v1/active-trips/passengers/in-transit - Set passenger to in-transit")
			activeTripPassengers.PUT("/in-transit", activeTripHandler.UpdatePassengerStatusToInTransit)

			logger.Info("  ✅ PUT /api/v1/active-trips/passengers/completed - Complete passenger journey")
			activeTripPassengers.PUT("/completed", activeTripHandler.UpdatePassengerStatusToCompleted)
		}
		logger.Info("👥 Active Trip Passenger Management routes registered successfully")

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

		// Admin Authentication routes (separate from regular user auth)
		logger.Info("🔐 Registering Admin Authentication routes...")
		adminAuth := v1.Group("/admin/auth")
		{
			// Public routes
			logger.Info("  ✅ POST /api/v1/admin/auth/login")
			adminAuth.POST("/login", adminAuthHandler.Login)
			logger.Info("  ✅ POST /api/v1/admin/auth/refresh")
			adminAuth.POST("/refresh", adminAuthHandler.RefreshToken)
			logger.Info("  ✅ POST /api/v1/admin/auth/logout")
			adminAuth.POST("/logout", adminAuthHandler.Logout)

			// Protected routes (require admin JWT authentication)
			adminProtected := adminAuth.Group("")
			adminProtected.Use(middleware.AuthMiddleware(jwtService))
			{
				logger.Info("  ✅ GET /api/v1/admin/auth/profile")
				adminProtected.GET("/profile", adminAuthHandler.GetProfile)
				logger.Info("  ✅ POST /api/v1/admin/auth/change-password")
				adminProtected.POST("/change-password", adminAuthHandler.ChangePassword)
				logger.Info("  ✅ POST /api/v1/admin/auth/create")
				adminProtected.POST("/create", adminAuthHandler.CreateAdmin)
				logger.Info("  ✅ GET /api/v1/admin/auth/list")
				adminProtected.GET("/list", adminAuthHandler.ListAdmins)
			}
		}
		logger.Info("🔐 Admin Authentication routes registered successfully")

		// Bus Seat Layout routes (admin only)
		logger.Info("🚌 Registering Bus Seat Layout routes...")
		busSeatLayout := v1.Group("/admin/seat-layouts")
		busSeatLayout.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ POST /api/v1/admin/seat-layouts")
			busSeatLayout.POST("", busSeatLayoutHandler.CreateTemplate)
			logger.Info("  ✅ GET /api/v1/admin/seat-layouts")
			busSeatLayout.GET("", busSeatLayoutHandler.ListTemplates)
			logger.Info("  ✅ GET /api/v1/admin/seat-layouts/:id")
			busSeatLayout.GET("/:id", busSeatLayoutHandler.GetTemplate)
			logger.Info("  ✅ PUT /api/v1/admin/seat-layouts/:id")
			busSeatLayout.PUT("/:id", busSeatLayoutHandler.UpdateTemplate)
			logger.Info("  ✅ DELETE /api/v1/admin/seat-layouts/:id")
			busSeatLayout.DELETE("/:id", busSeatLayoutHandler.DeleteTemplate)
		}
		logger.Info("🚌 Bus Seat Layout routes registered successfully")

		// User routes (protected)
		user := v1.Group("/user")
		user.Use(middleware.AuthMiddleware(jwtService))
		{
			user.GET("/profile", authHandler.GetProfile)
			user.PUT("/profile", authHandler.UpdateProfile)
			user.POST("/complete-basic-profile", authHandler.CompleteBasicProfile) // Simple first_name + last_name for passengers
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
				staffProtected.GET("/my-trips", staffHandler.GetMyTrips)

				// Active Trip routes (Start Trip / End Trip / Location tracking)
				logger.Info("🚌 Registering Active Trip routes...")
				staffProtected.GET("/trips/my-active", activeTripHandler.GetMyActiveTrip)
				staffProtected.POST("/trips/start", activeTripHandler.StartTrip)
				staffProtected.PUT("/trips/:id/location", activeTripHandler.UpdateLocation)
				staffProtected.POST("/trips/:id/end", activeTripHandler.EndTrip)
				staffProtected.GET("/trips/:id/active", activeTripHandler.GetActiveTrip)
				staffProtected.PUT("/trips/:id/passengers", activeTripHandler.UpdatePassengerCount)
				staffProtected.GET("/trips/:id/bookings", staffBookingHandler.GetTripBookings)
				logger.Info("✓ Active Trip routes registered")
			}
		}

		// Bus Owner routes (all protected)
		busOwner := v1.Group("/bus-owner")
		busOwner.Use(middleware.AuthMiddleware(jwtService))
		{
			busOwner.GET("/profile", busOwnerHandler.GetProfile)
			busOwner.GET("/profile-status", busOwnerHandler.CheckProfileStatus)
			busOwner.POST("/complete-onboarding", busOwnerHandler.CompleteOnboarding)
			busOwner.GET("/staff", busOwnerHandler.GetStaff)            // Get all staff (drivers & conductors)
			busOwner.POST("/staff", busOwnerHandler.AddStaff)           // Add driver or conductor (legacy - creates new)
			busOwner.POST("/staff/verify", busOwnerHandler.VerifyStaff) // Verify if staff can be added
			busOwner.POST("/staff/link", busOwnerHandler.LinkStaff)     // Link verified staff to bus owner
			busOwner.POST("/staff/unlink", busOwnerHandler.UnlinkStaff) // Remove staff from bus owner (end employment)
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
			logger.Info("  ✅ GET /api/v1/lounges/active (public)")
			lounges.GET("/active", loungeHandler.GetAllActiveLounges)
			logger.Info("  ✅ GET /api/v1/lounges/states (public)")
			lounges.GET("/states", loungeHandler.GetDistinctStates)
			logger.Info("  ✅ GET /api/v1/lounges/by-stop/:stopId (public)")
			lounges.GET("/by-stop/:stopId", loungeHandler.GetLoungesByStop)
			logger.Info("  ✅ GET /api/v1/lounges/by-route/:routeId (public)")
			lounges.GET("/by-route/:routeId", loungeHandler.GetLoungesByRoute)
			logger.Info("  ✅ GET /api/v1/lounges/near-stop/:routeId/:stopId (public)")
			lounges.GET("/near-stop/:routeId/:stopId", loungeHandler.GetLoungesNearStop)

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
				// Permission management moved to users.roles array - removed permission_type field
				logger.Info("  ✅ PUT /api/v1/lounges/:id/staff/:staff_id/status")
				loungesProtected.PUT("/:id/staff/:staff_id/status", loungeStaffHandler.UpdateStaffStatus)
				logger.Info("  ✅ DELETE /api/v1/lounges/:id/staff/:staff_id")
				loungesProtected.DELETE("/:id/staff/:staff_id", loungeStaffHandler.RemoveStaff)
			}
		}
		logger.Info("� Lounge routes registered successfully")

		// ============================================================================
		// LOUNGE BOOKING & MARKETPLACE ROUTES
		// ============================================================================
		logger.Info("🏨 Registering Lounge Booking routes...")

		// Lounge Marketplace - Categories (public)
		loungeMarketplace := v1.Group("/lounge-marketplace")
		{
			logger.Info("  ✅ GET /api/v1/lounge-marketplace/categories (public)")
			loungeMarketplace.GET("/categories", loungeBookingHandler.GetCategories)
		}

		// Lounge Products - Add to existing lounges group (protected)
		loungesProtectedProducts := v1.Group("/lounges")
		loungesProtectedProducts.Use(middleware.AuthMiddleware(jwtService))
		{
			// Products for a lounge (anyone can view, owner can manage)
			logger.Info("  ✅ GET /api/v1/lounges/:id/products")
			loungesProtectedProducts.GET("/:id/products", loungeBookingHandler.GetLoungeProducts)
			logger.Info("  ✅ POST /api/v1/lounges/:id/products (owner only)")
			loungesProtectedProducts.POST("/:id/products", loungeBookingHandler.CreateProduct)
			logger.Info("  ✅ PUT /api/v1/lounges/:id/products/:product_id (owner only)")
			loungesProtectedProducts.PUT("/:id/products/:product_id", loungeBookingHandler.UpdateProduct)
			logger.Info("  ✅ DELETE /api/v1/lounges/:id/products/:product_id (owner only)")
			loungesProtectedProducts.DELETE("/:id/products/:product_id", loungeBookingHandler.DeleteProduct)

			// Bookings for a lounge (owner/staff view)
			logger.Info("  ✅ GET /api/v1/lounges/:id/bookings (owner/staff)")
			loungesProtectedProducts.GET("/:id/bookings", loungeBookingHandler.GetLoungeBookingsForOwner)
			logger.Info("  ✅ GET /api/v1/lounges/:id/bookings/today (owner/staff)")
			loungesProtectedProducts.GET("/:id/bookings/today", loungeBookingHandler.GetTodaysBookings)
		}

		// Lounge Bookings - Passenger endpoints
		loungeBookings := v1.Group("/lounge-bookings")
		loungeBookings.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ POST /api/v1/lounge-bookings - Create lounge booking")
			loungeBookings.POST("", loungeBookingHandler.CreateLoungeBooking)
			logger.Info("  ✅ GET /api/v1/lounge-bookings - Get my lounge bookings")
			loungeBookings.GET("", loungeBookingHandler.GetMyLoungeBookings)
			logger.Info("  ✅ GET /api/v1/lounge-bookings/upcoming - Get upcoming bookings")
			loungeBookings.GET("/upcoming", loungeBookingHandler.GetUpcomingLoungeBookings)
			logger.Info("  ✅ GET /api/v1/lounge-bookings/:id - Get booking by ID")
			loungeBookings.GET("/:id", loungeBookingHandler.GetLoungeBookingByID)
			logger.Info("  ✅ GET /api/v1/lounge-bookings/reference/:reference - Get by reference")
			loungeBookings.GET("/reference/:reference", loungeBookingHandler.GetLoungeBookingByReference)
			logger.Info("  ✅ POST /api/v1/lounge-bookings/:id/cancel - Cancel booking")
			loungeBookings.POST("/:id/cancel", loungeBookingHandler.CancelLoungeBooking)

			// Staff operations
			logger.Info("  ✅ POST /api/v1/lounge-bookings/:id/check-in - Check in guest")
			loungeBookings.POST("/:id/check-in", loungeBookingHandler.CheckInGuest)
			logger.Info("  ✅ POST /api/v1/lounge-bookings/:id/complete - Complete booking")
			loungeBookings.POST("/:id/complete", loungeBookingHandler.CompleteLoungeBooking)

			// Orders for a booking
			logger.Info("  ✅ GET /api/v1/lounge-bookings/:id/orders - Get booking orders")
			loungeBookings.GET("/:id/orders", loungeBookingHandler.GetBookingOrders)
		}

		// Lounge Orders - In-lounge ordering
		loungeOrders := v1.Group("/lounge-orders")
		loungeOrders.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ POST /api/v1/lounge-orders - Create in-lounge order")
			loungeOrders.POST("", loungeBookingHandler.CreateLoungeOrder)
			logger.Info("  ✅ PUT /api/v1/lounge-orders/:id/status - Update order status")
			loungeOrders.PUT("/:id/status", loungeBookingHandler.UpdateOrderStatus)
		}
		logger.Info("🏨 Lounge Booking routes registered successfully")

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

			// NEW: Publish/Unpublish endpoints
			scheduledTrips.PUT("/:id/publish", scheduledTripHandler.PublishTrip)
			scheduledTrips.PUT("/:id/unpublish", scheduledTripHandler.UnpublishTrip)
			scheduledTrips.POST("/bulk-publish", scheduledTripHandler.BulkPublishTrips)
			scheduledTrips.POST("/bulk-unpublish", scheduledTripHandler.BulkUnpublishTrips)

			// NEW: Assign staff and permit
			scheduledTrips.PATCH("/:id/assign", scheduledTripHandler.AssignStaffAndPermit)
			// NEW: Assign seat layout
			scheduledTrips.PATCH("/:id/assign-seat-layout", scheduledTripHandler.AssignSeatLayout)

			// ============================================================================
			// TRIP SEATS ROUTES (Seat management for scheduled trips)
			// ============================================================================
			scheduledTrips.GET("/:id/seats", tripSeatHandler.GetTripSeats)
			scheduledTrips.GET("/:id/seats/summary", tripSeatHandler.GetTripSeatSummary)
			scheduledTrips.POST("/:id/seats/create", tripSeatHandler.CreateTripSeats)
			scheduledTrips.POST("/:id/seats/block", tripSeatHandler.BlockSeats)
			scheduledTrips.POST("/:id/seats/unblock", tripSeatHandler.UnblockSeats)
			scheduledTrips.PUT("/:id/seats/price", tripSeatHandler.UpdateSeatPrices)

			// Route stops for manual booking dropdown
			scheduledTrips.GET("/:id/route-stops", tripSeatHandler.GetTripRouteStops)

			// ============================================================================
			// MANUAL BOOKINGS ROUTES (Phone/Agent/Walk-in bookings)
			// ============================================================================
			scheduledTrips.GET("/:id/manual-bookings", tripSeatHandler.GetManualBookings)
			scheduledTrips.POST("/:id/manual-bookings", tripSeatHandler.CreateManualBooking)
		}

		// Manual Bookings standalone routes (for operations on existing bookings)
		logger.Info("📋 Registering Manual Booking routes...")
		manualBookings := v1.Group("/manual-bookings")
		manualBookings.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ GET /api/v1/manual-bookings/:id")
			manualBookings.GET("/:id", tripSeatHandler.GetManualBooking)
			logger.Info("  ✅ GET /api/v1/manual-bookings/reference/:ref")
			manualBookings.GET("/reference/:ref", tripSeatHandler.GetManualBookingByReference)
			logger.Info("  ✅ PUT /api/v1/manual-bookings/:id/payment")
			manualBookings.PUT("/:id/payment", tripSeatHandler.UpdateManualBookingPayment)
			logger.Info("  ✅ PUT /api/v1/manual-bookings/:id/status")
			manualBookings.PUT("/:id/status", tripSeatHandler.UpdateManualBookingStatus)
			logger.Info("  ✅ DELETE /api/v1/manual-bookings/:id")
			manualBookings.DELETE("/:id", tripSeatHandler.CancelManualBooking)
			logger.Info("  ✅ GET /api/v1/manual-bookings/search")
			manualBookings.GET("/search", tripSeatHandler.SearchManualBookingsByPhone)
		}
		logger.Info("📋 Manual Booking routes registered successfully")

		// ============================================================================
		// APP BOOKINGS ROUTES (Passenger app bookings)
		// ============================================================================
		logger.Info("📱 Registering App Booking routes...")
		appBookings := v1.Group("/bookings")
		appBookings.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ POST /api/v1/bookings - Create new booking")
			appBookings.POST("", appBookingHandler.CreateBooking)
			logger.Info("  ✅ GET /api/v1/bookings - Get my bookings")
			appBookings.GET("", appBookingHandler.GetMyBookings)
			logger.Info("  ✅ GET /api/v1/bookings/upcoming - Get upcoming bookings")
			appBookings.GET("/upcoming", appBookingHandler.GetUpcomingBookings)
			logger.Info("  ✅ GET /api/v1/bookings/:id - Get booking by ID")
			appBookings.GET("/:id", appBookingHandler.GetBookingByID)
			logger.Info("  ✅ GET /api/v1/bookings/reference/:reference - Get booking by reference")
			appBookings.GET("/reference/:reference", appBookingHandler.GetBookingByReference)
			logger.Info("  ✅ POST /api/v1/bookings/:id/confirm-payment - Confirm payment")
			appBookings.POST("/:id/confirm-payment", appBookingHandler.ConfirmPayment)
			logger.Info("  ✅ POST /api/v1/bookings/:id/cancel - Cancel booking")
			appBookings.POST("/:id/cancel", appBookingHandler.CancelBooking)
			logger.Info("  ✅ GET /api/v1/bookings/:id/qr - Get booking QR code")
			appBookings.GET("/:id/qr", appBookingHandler.GetBookingQR)
		}
		logger.Info("📱 App Booking routes registered successfully")

		// ============================================================================
		// BOOKING ORCHESTRATION ROUTES (Intent → Payment → Confirm)
		// ============================================================================
		logger.Info("🎯 Registering Booking Orchestration routes...")

		// Booking Intent routes (protected - requires auth)
		bookingOrchestration := v1.Group("/booking")
		bookingOrchestration.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ POST /api/v1/booking/intent - Create booking intent")
			bookingOrchestration.POST("/intent", bookingOrchestratorHandler.CreateIntent)

			logger.Info("  ✅ GET /api/v1/booking/intents - Get my intents")
			bookingOrchestration.GET("/intents", bookingOrchestratorHandler.GetMyIntents)

			logger.Info("  ✅ GET /api/v1/booking/intent/:intent_id - Get intent status")
			bookingOrchestration.GET("/intent/:intent_id", bookingOrchestratorHandler.GetIntentStatus)

			logger.Info("  ✅ POST /api/v1/booking/intent/:intent_id/initiate-payment - Initiate payment")
			bookingOrchestration.POST("/intent/:intent_id/initiate-payment", bookingOrchestratorHandler.InitiatePayment)

			logger.Info("  ✅ POST /api/v1/booking/intent/:intent_id/cancel - Cancel intent")
			bookingOrchestration.POST("/intent/:intent_id/cancel", bookingOrchestratorHandler.CancelIntent)

			logger.Info("  ✅ PATCH /api/v1/booking/intent/:intent_id/add-lounge - Add lounge to intent")
			bookingOrchestration.PATCH("/intent/:intent_id/add-lounge", bookingOrchestratorHandler.AddLoungeToIntent)

			logger.Info("  ✅ POST /api/v1/booking/confirm - Confirm booking after payment")
			bookingOrchestration.POST("/confirm", bookingOrchestratorHandler.ConfirmBooking)
		}

		// Payment webhook (no auth - called by payment gateway)
		logger.Info("  ✅ POST /api/v1/payments/webhook - Payment gateway webhook")
		v1.POST("/payments/webhook", bookingOrchestratorHandler.PaymentWebhook)

		// Payment return URL (no auth - browser redirect from payment gateway)
		logger.Info("  ✅ GET /api/v1/payments/return - Payment return page")
		v1.GET("/payments/return", bookingOrchestratorHandler.PaymentReturn)

		logger.Info("🎯 Booking Orchestration routes registered successfully")

		// ============================================================================
		// STAFF BOOKING ROUTES (Conductor/Driver operations)
		// ============================================================================
		logger.Info("👨‍✈️ Registering Staff Booking routes...")
		staffBookings := v1.Group("/staff/bookings")
		staffBookings.Use(middleware.AuthMiddleware(jwtService))
		{
			logger.Info("  ✅ POST /api/v1/staff/bookings/verify - Verify booking by QR")
			staffBookings.POST("/verify", staffBookingHandler.VerifyBookingByQR)
			logger.Info("  ✅ POST /api/v1/staff/bookings/check-in - Check-in passenger")
			staffBookings.POST("/check-in", staffBookingHandler.CheckInPassenger)
			logger.Info("  ✅ POST /api/v1/staff/bookings/board - Board passenger")
			staffBookings.POST("/board", staffBookingHandler.BoardPassenger)
			logger.Info("  ✅ POST /api/v1/staff/bookings/no-show - Mark no-show")
			staffBookings.POST("/no-show", staffBookingHandler.MarkNoShow)
		}
		logger.Info("👨‍✈️ Staff Booking routes registered successfully")

		// Permit-specific trip routes
		permits.GET("/:id/trip-schedules", tripScheduleHandler.GetSchedulesByPermit)
		permits.GET("/:id/scheduled-trips", scheduledTripHandler.GetTripsByPermit)

		// Public bookable trips (no auth required)
		v1.GET("/bookable-trips", scheduledTripHandler.GetBookableTrips)

		// ============================================================================
		// SEARCH ROUTES (Phase 1 MVP - Trip Discovery)
		// ============================================================================
		logger.Info("🔍 Registering Search routes...")

		// Public search routes (no authentication required)
		search := v1.Group("/search")
		{
			logger.Info("  ✅ POST /api/v1/search - Main search endpoint")
			search.POST("", searchHandler.SearchTrips)

			logger.Info("  ✅ GET /api/v1/search/popular - Popular routes")
			search.GET("/popular", searchHandler.GetPopularRoutes)

			logger.Info("  ✅ GET /api/v1/search/autocomplete - Stop suggestions")
			search.GET("/autocomplete", searchHandler.GetStopAutocomplete)

			logger.Info("  ✅ GET /api/v1/search/health - Health check")
			search.GET("/health", searchHandler.HealthCheck)
		}
		logger.Info("🔍 Search routes registered successfully")

		// System Settings routes (protected)
		systemSettings := v1.Group("/system-settings")
		systemSettings.Use(middleware.AuthMiddleware(jwtService))
		{
			systemSettings.GET("", systemSettingHandler.GetAllSettings)
			systemSettings.GET("/:key", systemSettingHandler.GetSettingByKey)
			systemSettings.PUT("/:key", systemSettingHandler.UpdateSetting)
		}

		// Admin routes
		admin := v1.Group("/admin")
		// TODO: Add admin auth middleware
		{
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

			// Search analytics
			admin.GET("/search/analytics", searchHandler.GetSearchAnalytics)
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
