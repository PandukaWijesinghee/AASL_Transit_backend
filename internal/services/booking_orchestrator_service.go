package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BookingOrchestratorConfig holds configuration for the orchestrator
type BookingOrchestratorConfig struct {
	IntentTTL       time.Duration // How long intents are valid (default 10 min)
	PaymentTimeout  time.Duration // How long to wait for payment (default 15 min)
	DefaultCurrency string        // Default currency (default LKR)
}

// DefaultOrchestratorConfig returns default configuration
func DefaultOrchestratorConfig() BookingOrchestratorConfig {
	return BookingOrchestratorConfig{
		IntentTTL:       10 * time.Minute,
		PaymentTimeout:  15 * time.Minute,
		DefaultCurrency: "LKR",
	}
}

// BookingOrchestratorService handles the Intent → Payment → Confirm booking flow
type BookingOrchestratorService struct {
	intentRepo        *database.BookingIntentRepository
	tripSeatRepo      *database.TripSeatRepository
	scheduledTripRepo *database.ScheduledTripRepository
	appBookingRepo    *database.AppBookingRepository
	loungeBookingRepo *database.LoungeBookingRepository
	loungeRepo        *database.LoungeRepository
	busOwnerRouteRepo *database.BusOwnerRouteRepository
	payableService    *PAYableService
	config            BookingOrchestratorConfig
	logger            *logrus.Logger
}

// NewBookingOrchestratorService creates a new orchestrator service
func NewBookingOrchestratorService(
	intentRepo *database.BookingIntentRepository,
	tripSeatRepo *database.TripSeatRepository,
	scheduledTripRepo *database.ScheduledTripRepository,
	appBookingRepo *database.AppBookingRepository,
	loungeBookingRepo *database.LoungeBookingRepository,
	loungeRepo *database.LoungeRepository,
	busOwnerRouteRepo *database.BusOwnerRouteRepository,
	payableService *PAYableService,
	config BookingOrchestratorConfig,
	logger *logrus.Logger,
) *BookingOrchestratorService {
	return &BookingOrchestratorService{
		intentRepo:        intentRepo,
		tripSeatRepo:      tripSeatRepo,
		scheduledTripRepo: scheduledTripRepo,
		appBookingRepo:    appBookingRepo,
		loungeBookingRepo: loungeBookingRepo,
		loungeRepo:        loungeRepo,
		busOwnerRouteRepo: busOwnerRouteRepo,
		payableService:    payableService,
		config:            config,
		logger:            logger,
	}
}

// ============================================================================
// CREATE INTENT (Phase 1)
// ============================================================================

// CreateIntent creates a new booking intent with TTL-based holds
func (s *BookingOrchestratorService) CreateIntent(
	userID uuid.UUID,
	req *models.CreateBookingIntentRequest,
) (*models.BookingIntentResponse, error) {
	// 1. Check idempotency key if provided
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		existing, err := s.intentRepo.GetIntentByIdempotencyKey(*req.IdempotencyKey, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
		if existing != nil {
			// Return existing intent
			return s.buildIntentResponse(existing), nil
		}
	}

	// 2. Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.config.IntentTTL)

	// 3. Build intent object
	intent := &models.BookingIntent{
		UserID:         userID,
		IntentType:     req.IntentType,
		Status:         models.IntentStatusHeld,
		Currency:       s.config.DefaultCurrency,
		PaymentGateway: "payable",
		ExpiresAt:      expiresAt,
		IdempotencyKey: req.IdempotencyKey,
	}

	// 4. Process bus intent (if present)
	if req.Bus != nil {
		busPayload, busFare, err := s.processBusIntent(req.Bus, expiresAt)
		if err != nil {
			return nil, err
		}
		intent.BusIntent = busPayload
		intent.BusFare = busFare
	}

	// 5. Process pre-trip lounge intent (if present)
	if req.PreTripLounge != nil {
		loungePayload, loungeFare, err := s.processLoungeIntent(req.PreTripLounge, intent.ID, expiresAt, "pre_trip")
		if err != nil {
			return nil, err
		}
		intent.PreTripLoungeIntent = loungePayload
		intent.PreLoungeFare = loungeFare
	}

	// 6. Process post-trip lounge intent (if present)
	if req.PostTripLounge != nil {
		loungePayload, loungeFare, err := s.processLoungeIntent(req.PostTripLounge, intent.ID, expiresAt, "post_trip")
		if err != nil {
			return nil, err
		}
		intent.PostTripLoungeIntent = loungePayload
		intent.PostLoungeFare = loungeFare
	}

	// 7. Calculate totals
	intent.TotalAmount = intent.BusFare + intent.PreLoungeFare + intent.PostLoungeFare
	intent.PricingSnapshot = models.PricingSnapshot{
		BusFare:        intent.BusFare,
		PreLoungeFare:  intent.PreLoungeFare,
		PostLoungeFare: intent.PostLoungeFare,
		Total:          intent.TotalAmount,
		Currency:       intent.Currency,
		CalculatedAt:   time.Now(),
	}

	// 8. Save intent to database
	if err := s.intentRepo.CreateIntent(intent); err != nil {
		// Rollback any holds we made
		s.rollbackHolds(intent.ID)
		return nil, fmt.Errorf("failed to create intent: %w", err)
	}

	// 9. Now that we have the intent ID, hold seats and lounge capacity
	if req.Bus != nil {
		seatIDs := make([]string, len(req.Bus.Seats))
		for i, seat := range req.Bus.Seats {
			seatIDs[i] = seat.TripSeatID
		}

		heldCount, err := s.intentRepo.HoldSeatsForIntent(intent.ID, seatIDs, expiresAt)
		if err != nil {
			s.rollbackHolds(intent.ID)
			s.intentRepo.UpdateIntentExpired(intent.ID)
			return nil, fmt.Errorf("failed to hold seats: %w", err)
		}

		if heldCount < len(seatIDs) {
			// Some seats couldn't be held - they were taken
			s.rollbackHolds(intent.ID)
			s.intentRepo.UpdateIntentExpired(intent.ID)

			// Find which seats were taken
			_, unavailable, _ := s.intentRepo.CheckSeatsAvailableForHold(seatIDs)
			return nil, s.buildPartialAvailabilityError(unavailable, nil, nil)
		}
	}

	// 10. Create lounge capacity holds
	if req.PreTripLounge != nil {
		err := s.createLoungeHold(intent.ID, req.PreTripLounge, expiresAt, "pre_trip")
		if err != nil {
			s.rollbackHolds(intent.ID)
			s.intentRepo.UpdateIntentExpired(intent.ID)
			return nil, err
		}
	}
	if req.PostTripLounge != nil {
		err := s.createLoungeHold(intent.ID, req.PostTripLounge, expiresAt, "post_trip")
		if err != nil {
			s.rollbackHolds(intent.ID)
			s.intentRepo.UpdateIntentExpired(intent.ID)
			return nil, err
		}
	}

	s.logger.WithFields(logrus.Fields{
		"intent_id":    intent.ID,
		"user_id":      userID,
		"intent_type":  intent.IntentType,
		"total_amount": intent.TotalAmount,
		"expires_at":   expiresAt,
	}).Info("Booking intent created successfully")

	return s.buildIntentResponse(intent), nil
}

// processBusIntent validates and processes bus intent, returns payload and fare
func (s *BookingOrchestratorService) processBusIntent(
	req *models.BusIntentRequest,
	expiresAt time.Time,
) (*models.BusIntentPayload, float64, error) {
	// 1. Get scheduled trip details
	trip, err := s.scheduledTripRepo.GetByID(req.ScheduledTripID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get scheduled trip: %w", err)
	}
	if trip == nil {
		return nil, 0, fmt.Errorf("scheduled trip not found")
	}

	// 2. Check trip is still bookable
	if trip.Status != models.ScheduledTripStatusScheduled && trip.Status != models.ScheduledTripStatusConfirmed {
		return nil, 0, fmt.Errorf("trip is not available for booking (status: %s)", trip.Status)
	}
	if trip.DepartureDatetime.Before(time.Now()) {
		return nil, 0, fmt.Errorf("trip has already departed")
	}

	// 3. Get seat IDs and check availability
	seatIDs := make([]string, len(req.Seats))
	for i, seat := range req.Seats {
		seatIDs[i] = seat.TripSeatID
	}

	available, unavailable, err := s.intentRepo.CheckSeatsAvailableForHold(seatIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check seat availability: %w", err)
	}
	if len(unavailable) > 0 {
		return nil, 0, s.buildPartialAvailabilityError(unavailable, nil, nil)
	}

	// 4. Get seat prices
	seats, err := s.tripSeatRepo.GetByIDs(available)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get seat details: %w", err)
	}

	// Build seat map for quick lookup
	seatMap := make(map[string]models.TripSeat)
	for _, seat := range seats {
		seatMap[seat.ID] = seat
	}

	// 5. Build payload with prices
	var totalFare float64
	intentSeats := make([]models.BusIntentSeat, len(req.Seats))
	for i, reqSeat := range req.Seats {
		seat, exists := seatMap[reqSeat.TripSeatID]
		if !exists {
			return nil, 0, fmt.Errorf("seat %s not found", reqSeat.TripSeatID)
		}

		intentSeats[i] = models.BusIntentSeat{
			TripSeatID:      reqSeat.TripSeatID,
			SeatNumber:      seat.SeatNumber,
			SeatType:        seat.SeatType,
			SeatPrice:       seat.SeatPrice,
			PassengerName:   reqSeat.PassengerName,
			PassengerPhone:  reqSeat.PassengerPhone,
			PassengerGender: reqSeat.PassengerGender,
			IsPrimary:       reqSeat.IsPrimary,
		}
		totalFare += seat.SeatPrice
	}

	// 6. Get trip info for display
	tripInfo := &models.BusIntentTripInfo{
		DepartureDatetime: trip.DepartureDatetime,
	}

	// Get route name
	if trip.BusOwnerRouteID != nil {
		route, err := s.busOwnerRouteRepo.GetByID(*trip.BusOwnerRouteID)
		if err == nil && route != nil {
			if route.MasterRouteID != "" {
				// Has master route - would need another lookup for route name
				tripInfo.RouteName = route.CustomRouteName
			} else {
				tripInfo.RouteName = route.CustomRouteName
			}
		}
	}

	payload := &models.BusIntentPayload{
		ScheduledTripID:   req.ScheduledTripID,
		BoardingStopID:    req.BoardingStopID,
		BoardingStopName:  req.BoardingStopName,
		AlightingStopID:   req.AlightingStopID,
		AlightingStopName: req.AlightingStopName,
		Seats:             intentSeats,
		PassengerName:     req.PassengerName,
		PassengerPhone:    req.PassengerPhone,
		PassengerEmail:    req.PassengerEmail,
		SpecialRequests:   req.SpecialRequests,
		TripInfo:          tripInfo,
	}

	return payload, totalFare, nil
}

// processLoungeIntent validates and processes lounge intent, returns payload and fare
func (s *BookingOrchestratorService) processLoungeIntent(
	req *models.LoungeIntentRequest,
	intentID uuid.UUID,
	expiresAt time.Time,
	loungeType string, // "pre_trip" or "post_trip"
) (*models.LoungeIntentPayload, float64, error) {
	// 1. Get lounge details
	loungeID, err := uuid.Parse(req.LoungeID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid lounge_id for %s lounge", loungeType)
	}

	lounge, err := s.loungeRepo.GetLoungeByID(loungeID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get lounge: %w", err)
	}
	if lounge == nil {
		return nil, 0, fmt.Errorf("lounge not found")
	}

	// 2. Get lounge price based on pricing type
	priceStr, err := s.loungeBookingRepo.GetLoungePrice(loungeID, req.PricingType)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get lounge price: %w", err)
	}

	var pricePerGuest float64
	fmt.Sscanf(priceStr, "%f", &pricePerGuest)

	// 3. Build guests list
	guests := make([]models.LoungeIntentGuest, len(req.Guests))
	for i, g := range req.Guests {
		guests[i] = models.LoungeIntentGuest{
			GuestName:  g.GuestName,
			GuestPhone: g.GuestPhone,
			IsPrimary:  i == 0, // First guest is primary
		}
	}
	guestCount := len(guests)

	// 4. Calculate lounge base price
	basePrice := pricePerGuest * float64(guestCount)

	// 5. Process pre-orders if any
	var preOrderTotal float64
	preOrders := make([]models.LoungeIntentPreOrder, 0)
	for _, po := range req.PreOrders {
		productID, err := uuid.Parse(po.ProductID)
		if err != nil {
			continue
		}
		product, err := s.loungeBookingRepo.GetProductByID(productID)
		if err != nil || product == nil {
			continue
		}

		var unitPrice float64
		fmt.Sscanf(product.Price, "%f", &unitPrice)

		preOrders = append(preOrders, models.LoungeIntentPreOrder{
			ProductID:   po.ProductID,
			ProductName: product.Name,
			ProductType: string(product.ProductType),
			ImageURL:    product.ImageURL,
			Quantity:    po.Quantity,
			UnitPrice:   unitPrice,
			TotalPrice:  unitPrice * float64(po.Quantity),
		})
		preOrderTotal += unitPrice * float64(po.Quantity)
	}

	totalPrice := basePrice + preOrderTotal

	// 6. Build payload
	payload := &models.LoungeIntentPayload{
		LoungeID:      req.LoungeID,
		LoungeName:    lounge.LoungeName,
		PricingType:   req.PricingType,
		GuestCount:    guestCount,
		Guests:        guests,
		PreOrders:     preOrders,
		PricePerGuest: pricePerGuest,
		BasePrice:     basePrice,
		PreOrderTotal: preOrderTotal,
		TotalPrice:    totalPrice,
	}

	return payload, totalPrice, nil
}

// createLoungeHold creates a lounge capacity hold
func (s *BookingOrchestratorService) createLoungeHold(
	intentID uuid.UUID,
	req *models.LoungeIntentRequest,
	expiresAt time.Time,
	loungeType string,
) error {
	loungeID, _ := uuid.Parse(req.LoungeID)
	guestCount := len(req.Guests)

	// For now, use current date. In production, this would come from trip info
	date := time.Now()
	timeSlotStart := "09:00"
	timeSlotEnd := "12:00"

	// Check capacity
	available, err := s.intentRepo.GetLoungeCapacityAvailable(loungeID, date, timeSlotStart, timeSlotEnd)
	if err != nil {
		return fmt.Errorf("failed to check lounge capacity: %w", err)
	}
	if available < guestCount {
		return fmt.Errorf("lounge does not have enough capacity (available: %d, requested: %d)", available, guestCount)
	}

	// Create hold
	hold := &models.LoungeCapacityHold{
		LoungeID:      loungeID,
		IntentID:      intentID,
		Date:          date,
		TimeSlotStart: timeSlotStart,
		TimeSlotEnd:   timeSlotEnd,
		GuestsCount:   guestCount,
		HeldUntil:     expiresAt,
	}

	return s.intentRepo.CreateLoungeCapacityHold(hold)
}

// ============================================================================
// INITIATE PAYMENT (Phase 2)
// ============================================================================

// InitiatePayment initiates payment for an intent
func (s *BookingOrchestratorService) InitiatePayment(
	intentID uuid.UUID,
	userID uuid.UUID,
) (*models.InitiatePaymentResponse, error) {
	// 1. Get intent
	intent, err := s.intentRepo.GetIntentByID(intentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get intent: %w", err)
	}
	if intent == nil {
		return nil, fmt.Errorf("intent not found")
	}

	// 2. Verify ownership
	if intent.UserID != userID {
		return nil, fmt.Errorf("unauthorized: intent belongs to another user")
	}

	// 3. Check can initiate payment
	if !intent.CanInitiatePayment() {
		if intent.IsExpired() {
			return nil, fmt.Errorf("intent has expired")
		}
		return nil, fmt.Errorf("intent is not in valid state for payment (status: %s)", intent.Status)
	}

	// 4. Generate payment reference (using intent ID as invoice ID)
	paymentRef := fmt.Sprintf("INT-%s", intent.ID.String()[:8])
	amountStr := fmt.Sprintf("%.2f", intent.TotalAmount)

	// 5. Update intent to payment_pending
	if err := s.intentRepo.UpdateIntentPaymentPending(intent.ID, paymentRef); err != nil {
		return nil, fmt.Errorf("failed to update intent: %w", err)
	}

	// 6. Build payment response
	var response *models.InitiatePaymentResponse

	// Check if PAYable service is configured
	if s.payableService != nil && s.payableService.IsConfigured() {
		// Use real PAYable integration
		payableParams := &InitiatePaymentParams{
			InvoiceID:        paymentRef,
			Amount:           amountStr,
			CurrencyCode:     intent.Currency,
			CustomerName:     intent.PassengerName,
			CustomerPhone:    intent.PassengerPhone,
			OrderDescription: fmt.Sprintf("Bus Booking - %s", paymentRef),
		}

		payableResp, err := s.payableService.InitiatePayment(payableParams)
		if err != nil {
			s.logger.WithError(err).Error("Failed to initiate PAYable payment")
			// Don't fail completely - return a response that allows retry
			return nil, fmt.Errorf("payment gateway error: %w", err)
		}

		response = &models.InitiatePaymentResponse{
			PaymentURL:      payableResp.PaymentPage,
			InvoiceID:       paymentRef,
			Amount:          amountStr,
			Currency:        intent.Currency,
			UID:             payableResp.UID,
			StatusIndicator: payableResp.StatusIndicator,
			ExpiresAt:       intent.ExpiresAt,
		}

		// Store UID and StatusIndicator for webhook verification
		if err := s.intentRepo.UpdateIntentPaymentUID(intent.ID, payableResp.UID, payableResp.StatusIndicator); err != nil {
			s.logger.WithError(err).Warn("Failed to store payment UID - webhook verification may fail")
		}

		s.logger.WithFields(logrus.Fields{
			"intent_id":    intentID,
			"payment_ref":  paymentRef,
			"amount":       intent.TotalAmount,
			"uid":          payableResp.UID,
			"payment_page": payableResp.PaymentPage,
			"environment":  s.payableService.GetEnvironment(),
		}).Info("PAYable payment initiated for booking intent")
	} else {
		// Development mode - return placeholder URL
		s.logger.Warn("PAYable service not configured - using placeholder payment URL")
		response = &models.InitiatePaymentResponse{
			PaymentURL: fmt.Sprintf("https://gateway.payable.lk/pay/%s", paymentRef),
			InvoiceID:  paymentRef,
			Amount:     amountStr,
			Currency:   intent.Currency,
			ExpiresAt:  intent.ExpiresAt,
		}

		s.logger.WithFields(logrus.Fields{
			"intent_id":   intentID,
			"payment_ref": paymentRef,
			"amount":      intent.TotalAmount,
			"mode":        "placeholder",
		}).Info("Payment initiated for booking intent (placeholder mode)")
	}

	return response, nil
}

// ============================================================================
// CONFIRM BOOKING (Phase 3)
// ============================================================================

// ConfirmBooking confirms a booking intent after payment
func (s *BookingOrchestratorService) ConfirmBooking(
	intentID uuid.UUID,
	userID uuid.UUID,
	paymentReference *string,
) (*models.ConfirmBookingResponse, error) {
	// 1. Get intent
	intent, err := s.intentRepo.GetIntentByID(intentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get intent: %w", err)
	}
	if intent == nil {
		return nil, fmt.Errorf("intent not found")
	}

	// Log the intent state for debugging - using Info level for visibility
	confirmFields := logrus.Fields{
		"intent_id":              intent.ID,
		"intent_type":            intent.IntentType,
		"status":                 intent.Status,
		"has_bus_intent":         intent.BusIntent != nil,
		"has_pre_lounge_intent":  intent.PreTripLoungeIntent != nil,
		"has_post_lounge_intent": intent.PostTripLoungeIntent != nil,
		"pre_lounge_fare":        intent.PreLoungeFare,
		"post_lounge_fare":       intent.PostLoungeFare,
		"total_amount":           intent.TotalAmount,
	}
	// Add lounge IDs if present for detailed diagnosis
	if intent.PreTripLoungeIntent != nil {
		confirmFields["pre_lounge_id"] = intent.PreTripLoungeIntent.LoungeID
		confirmFields["pre_lounge_name"] = intent.PreTripLoungeIntent.LoungeName
	}
	if intent.PostTripLoungeIntent != nil {
		confirmFields["post_lounge_id"] = intent.PostTripLoungeIntent.LoungeID
	}
	s.logger.WithFields(confirmFields).Info("ConfirmBooking: Retrieved intent for confirmation")

	// 2. Verify ownership
	if intent.UserID != userID {
		return nil, fmt.Errorf("unauthorized: intent belongs to another user")
	}

	// 3. Check if already confirmed
	if intent.Status == models.IntentStatusConfirmed {
		// Return existing bookings (idempotent)
		return s.buildConfirmResponse(intent), nil
	}

	// 4. Check can confirm
	if !intent.CanConfirm() {
		if intent.IsExpired() {
			return nil, fmt.Errorf("intent has expired, seats have been released")
		}
		return nil, fmt.Errorf("intent cannot be confirmed (status: %s)", intent.Status)
	}

	// 5. Verify payment (in production, would check with gateway)
	// For now, we trust the payment reference
	if paymentReference != nil && *paymentReference != "" {
		if err := s.intentRepo.UpdateIntentPaymentSuccess(intent.ID); err != nil {
			s.logger.WithError(err).Warn("Failed to update payment status")
		}
	}

	// 6. Mark as confirming
	if err := s.intentRepo.UpdateIntentStatus(intent.ID, models.IntentStatusConfirming); err != nil {
		return nil, fmt.Errorf("failed to update intent status: %w", err)
	}

	// 7. Create actual bookings in a transaction
	var busBookingID, preLoungeBookingID, postLoungeBookingID *uuid.UUID
	var masterRef string

	// Create bus booking if present
	if intent.BusIntent != nil {
		busBooking, bookingRef, err := s.createBusBookingFromIntent(intent)
		if err != nil {
			// Mark as confirmation failed
			s.intentRepo.UpdateIntentConfirmationFailed(intent.ID)
			return nil, fmt.Errorf("failed to create bus booking: %w", err)
		}
		busBookingUUID, _ := uuid.Parse(busBooking.ID)
		busBookingID = &busBookingUUID
		masterRef = bookingRef
	}

	// Create pre-trip lounge booking if present
	if intent.PreTripLoungeIntent != nil {
		s.logger.WithFields(logrus.Fields{
			"intent_id":   intent.ID,
			"lounge_id":   intent.PreTripLoungeIntent.LoungeID,
			"lounge_name": intent.PreTripLoungeIntent.LoungeName,
			"total_price": intent.PreTripLoungeIntent.TotalPrice,
		}).Info("Creating pre-trip lounge booking from intent")

		preLoungeBooking, err := s.createLoungeBookingFromIntent(intent, intent.PreTripLoungeIntent, "pre_trip", busBookingID)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"error":     err.Error(),
				"intent_id": intent.ID,
				"lounge_id": intent.PreTripLoungeIntent.LoungeID,
			}).Error("Failed to create pre-trip lounge booking")
			// Continue - bus booking is created
		} else {
			id := preLoungeBooking.ID
			preLoungeBookingID = &id
			s.logger.WithFields(logrus.Fields{
				"pre_lounge_booking_id": id,
				"booking_reference":     preLoungeBooking.BookingReference,
			}).Info("Pre-trip lounge booking created successfully")
			if masterRef == "" {
				masterRef = preLoungeBooking.BookingReference
			}
		}
	} else {
		s.logger.WithField("intent_id", intent.ID).Info("No pre-trip lounge intent found - skipping lounge booking creation")
	}

	// Create post-trip lounge booking if present
	if intent.PostTripLoungeIntent != nil {
		postLoungeBooking, err := s.createLoungeBookingFromIntent(intent, intent.PostTripLoungeIntent, "post_trip", busBookingID)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"error":     err.Error(),
				"intent_id": intent.ID,
				"lounge_id": intent.PostTripLoungeIntent.LoungeID,
			}).Error("Failed to create post-trip lounge booking")
		} else {
			id := postLoungeBooking.ID
			postLoungeBookingID = &id
			if masterRef == "" {
				masterRef = postLoungeBooking.BookingReference
			}
		}
	}

	// 8. Mark intent as confirmed
	if err := s.intentRepo.UpdateIntentConfirmed(intent.ID, busBookingID, preLoungeBookingID, postLoungeBookingID); err != nil {
		return nil, fmt.Errorf("failed to mark intent as confirmed: %w", err)
	}

	// 9. Confirm lounge holds (convert from held to confirmed)
	s.intentRepo.ConfirmLoungeHoldsForIntent(intent.ID)

	// 10. Update lounge booking statuses and payment status to confirmed/paid
	if preLoungeBookingID != nil {
		if err := s.loungeBookingRepo.UpdateLoungeBookingStatus(*preLoungeBookingID, models.LoungeBookingStatusConfirmed); err != nil {
			s.logger.WithError(err).WithField("lounge_booking_id", preLoungeBookingID).Error("Failed to update pre-lounge booking status")
		}
		if err := s.loungeBookingRepo.UpdatePaymentStatus(*preLoungeBookingID, models.LoungePaymentPaid); err != nil {
			s.logger.WithError(err).WithField("lounge_booking_id", preLoungeBookingID).Error("Failed to update pre-lounge payment status")
		}
	}
	if postLoungeBookingID != nil {
		if err := s.loungeBookingRepo.UpdateLoungeBookingStatus(*postLoungeBookingID, models.LoungeBookingStatusConfirmed); err != nil {
			s.logger.WithError(err).WithField("lounge_booking_id", postLoungeBookingID).Error("Failed to update post-lounge booking status")
		}
		if err := s.loungeBookingRepo.UpdatePaymentStatus(*postLoungeBookingID, models.LoungePaymentPaid); err != nil {
			s.logger.WithError(err).WithField("lounge_booking_id", postLoungeBookingID).Error("Failed to update post-lounge payment status")
		}
	}

	// 11. Refresh intent to get booking IDs
	intent, _ = s.intentRepo.GetIntentByID(intentID)

	s.logger.WithFields(logrus.Fields{
		"intent_id":              intentID,
		"master_reference":       masterRef,
		"bus_booking_id":         busBookingID,
		"pre_lounge_booking_id":  preLoungeBookingID,
		"post_lounge_booking_id": postLoungeBookingID,
	}).Info("Booking confirmed successfully")

	return s.buildConfirmResponse(intent), nil
}

// createBusBookingFromIntent creates a bus booking from intent data
func (s *BookingOrchestratorService) createBusBookingFromIntent(intent *models.BookingIntent) (*models.BusBooking, string, error) {
	busIntent := intent.BusIntent

	// Build master booking
	masterBooking := &models.MasterBooking{
		UserID:         intent.UserID.String(),
		BookingType:    models.BookingTypeBusOnly,
		BusTotal:       intent.BusFare,
		Subtotal:       intent.BusFare,
		TotalAmount:    intent.BusFare,
		PaymentStatus:  models.MasterPaymentPaid, // Paid via intent
		BookingStatus:  models.MasterBookingConfirmed,
		PassengerName:  busIntent.PassengerName,
		PassengerPhone: busIntent.PassengerPhone,
		PassengerEmail: busIntent.PassengerEmail,
		BookingSource:  models.BookingSourceApp,
	}

	// Build bus booking
	busBooking := &models.BusBooking{
		ScheduledTripID: busIntent.ScheduledTripID,
		BoardingStopID:  busIntent.BoardingStopID,
		AlightingStopID: busIntent.AlightingStopID,
		NumberOfSeats:   len(busIntent.Seats),
		FarePerSeat:     intent.BusFare / float64(len(busIntent.Seats)),
		TotalFare:       intent.BusFare,
		Status:          models.BusBookingConfirmed,
	}
	if busIntent.SpecialRequests != nil {
		busBooking.SpecialRequests = busIntent.SpecialRequests
	}

	// Build seats
	seats := make([]models.BusBookingSeat, len(busIntent.Seats))
	for i, intentSeat := range busIntent.Seats {
		seats[i] = models.BusBookingSeat{
			TripSeatID:         &intentSeat.TripSeatID,
			PassengerName:      intentSeat.PassengerName,
			PassengerPhone:     intentSeat.PassengerPhone,
			PassengerGender:    intentSeat.PassengerGender,
			IsPrimaryPassenger: intentSeat.IsPrimary,
			Status:             models.SeatBookingBooked,
			SeatNumber:         intentSeat.SeatNumber,
			SeatType:           intentSeat.SeatType,
			SeatPrice:          intentSeat.SeatPrice,
		}
	}

	// Create booking
	response, err := s.appBookingRepo.CreateBooking(masterBooking, busBooking, seats, s.tripSeatRepo)
	if err != nil {
		return nil, "", err
	}

	// Clear seat holds (they are now booked)
	s.intentRepo.ReleaseSeatHoldsForIntent(intent.ID)

	return response.BusBooking, response.Booking.BookingReference, nil
}

// createLoungeBookingFromIntent creates a lounge booking from intent data
func (s *BookingOrchestratorService) createLoungeBookingFromIntent(
	intent *models.BookingIntent,
	loungeIntent *models.LoungeIntentPayload,
	bookingType string,
	busBookingID *uuid.UUID,
) (*models.LoungeBooking, error) {
	loungeID, _ := uuid.Parse(loungeIntent.LoungeID)

	// Build lounge booking
	booking := &models.LoungeBooking{
		UserID:           intent.UserID,
		LoungeID:         loungeID,
		BusBookingID:     busBookingID,
		ScheduledArrival: time.Now().Add(time.Hour), // Would be calculated from trip time
		NumberOfGuests:   loungeIntent.GuestCount,
		PricingType:      loungeIntent.PricingType,
		PricePerGuest:    fmt.Sprintf("%.2f", loungeIntent.PricePerGuest),
		BasePrice:        fmt.Sprintf("%.2f", loungeIntent.BasePrice),
		PreOrderTotal:    fmt.Sprintf("%.2f", loungeIntent.PreOrderTotal),
		DiscountAmount:   "0.00", // Default to zero discount
		TotalAmount:      fmt.Sprintf("%.2f", loungeIntent.TotalPrice),
		LoungeName:       loungeIntent.LoungeName,
		PrimaryGuestName: loungeIntent.Guests[0].GuestName,
	}

	if loungeIntent.Guests[0].GuestPhone != nil {
		booking.PrimaryGuestPhone = *loungeIntent.Guests[0].GuestPhone
	}

	// Set booking type
	switch bookingType {
	case "pre_trip":
		booking.BookingType = models.LoungeBookingPreTrip
	case "post_trip":
		booking.BookingType = models.LoungeBookingPostTrip
	default:
		booking.BookingType = models.LoungeBookingStandalone
	}

	// Build guests
	guests := make([]models.LoungeBookingGuest, len(loungeIntent.Guests))
	for i, g := range loungeIntent.Guests {
		guests[i] = models.LoungeBookingGuest{
			GuestName:      g.GuestName,
			IsPrimaryGuest: g.IsPrimary,
		}
	}

	// Build pre-orders
	preOrders := make([]models.LoungeBookingPreOrder, len(loungeIntent.PreOrders))
	for i, po := range loungeIntent.PreOrders {
		productID, _ := uuid.Parse(po.ProductID)
		preOrders[i] = models.LoungeBookingPreOrder{
			ProductID:   productID,
			ProductName: po.ProductName,
			ProductType: po.ProductType,
			Quantity:    po.Quantity,
			UnitPrice:   fmt.Sprintf("%.2f", po.UnitPrice),
			TotalPrice:  fmt.Sprintf("%.2f", po.TotalPrice),
		}
	}

	// Create booking
	return s.loungeBookingRepo.CreateLoungeBooking(booking, guests, preOrders)
}

// ============================================================================
// GET INTENT STATUS
// ============================================================================

// GetIntentStatus returns the current status of an intent
func (s *BookingOrchestratorService) GetIntentStatus(
	intentID uuid.UUID,
	userID uuid.UUID,
) (*models.GetIntentStatusResponse, error) {
	intent, err := s.intentRepo.GetIntentByID(intentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get intent: %w", err)
	}
	if intent == nil {
		return nil, fmt.Errorf("intent not found")
	}

	// Verify ownership
	if intent.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	response := &models.GetIntentStatusResponse{
		IntentID:      intent.ID,
		Status:        intent.Status,
		PaymentStatus: intent.PaymentStatus,
		PriceBreakdown: models.PriceBreakdown{
			BusFare:        intent.BusFare,
			PreLoungeFare:  intent.PreLoungeFare,
			PostLoungeFare: intent.PostLoungeFare,
			Total:          intent.TotalAmount,
			Currency:       intent.Currency,
		},
		ExpiresAt: intent.ExpiresAt,
		IsExpired: intent.IsExpired(),
	}

	// Include bookings if confirmed
	if intent.Status == models.IntentStatusConfirmed {
		response.Bookings = s.buildConfirmResponse(intent)
	}

	return response, nil
}

// ============================================================================
// GET INTENT BY PAYMENT UID (for webhook processing)
// ============================================================================

// GetIntentByPaymentUID retrieves an intent by its PAYable payment UID
func (s *BookingOrchestratorService) GetIntentByPaymentUID(uid string) (*models.BookingIntent, error) {
	return s.intentRepo.GetIntentByPaymentUID(uid)
}

// ============================================================================
// ADD LOUNGE TO EXISTING INTENT
// ============================================================================

// AddLoungeToIntentRequest represents a request to add lounge(s) to an existing intent
type AddLoungeToIntentRequest struct {
	IntentID       uuid.UUID                   `json:"intent_id"`
	PreTripLounge  *models.LoungeIntentPayload `json:"pre_trip_lounge,omitempty"`
	PostTripLounge *models.LoungeIntentPayload `json:"post_trip_lounge,omitempty"`
}

// AddLoungeToIntent adds pre-trip and/or post-trip lounge to an existing bus intent
// This keeps the seat hold active and extends the expiration time
func (s *BookingOrchestratorService) AddLoungeToIntent(
	intentID uuid.UUID,
	userID uuid.UUID,
	preTripLounge *models.LoungeIntentPayload,
	postTripLounge *models.LoungeIntentPayload,
) (*models.BookingIntentResponse, error) {
	// 1. Get and validate intent
	intent, err := s.intentRepo.GetIntentByID(intentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get intent: %w", err)
	}
	if intent == nil {
		return nil, fmt.Errorf("intent not found")
	}

	// Verify ownership
	if intent.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	// Check status - can only add lounges to held intents
	if intent.Status != models.IntentStatusHeld {
		return nil, fmt.Errorf("can only add lounges to held intents, current status: %s", intent.Status)
	}

	// Check if expired
	if time.Now().After(intent.ExpiresAt) {
		s.intentRepo.UpdateIntentExpired(intent.ID)
		return nil, fmt.Errorf("intent has expired")
	}

	// Helper function to calculate checkout time from pricing type
	calculateCheckoutTime := func(checkInTime string, pricingType string) string {
		// Parse check-in time
		t, err := time.Parse("15:04", checkInTime)
		if err != nil {
			return checkInTime // Fallback to same time
		}

		// Add hours based on pricing type
		var duration time.Duration
		switch pricingType {
		case "1_hour":
			duration = 1 * time.Hour
		case "2_hours":
			duration = 2 * time.Hour
		case "3_hours":
			duration = 3 * time.Hour
		case "until_bus":
			duration = 2 * time.Hour // Default for until_bus
		default:
			duration = 2 * time.Hour
		}

		checkout := t.Add(duration)
		return checkout.Format("15:04")
	}

	// Helper to parse lounge date
	parseLoungeDate := func(dateStr string) time.Time {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return time.Now()
		}
		return parsed
	}

	// 2. Calculate additional lounge fares
	var preLoungeFare, postLoungeFare float64

	if preTripLounge != nil {
		preLoungeFare = preTripLounge.TotalPrice
		// Create lounge capacity hold using actual lounge date/time
		expiresAt := time.Now().Add(s.config.IntentTTL)
		loungeID, _ := uuid.Parse(preTripLounge.LoungeID)

		loungeDate := parseLoungeDate(preTripLounge.Date)
		checkInTime := preTripLounge.CheckInTime
		if checkInTime == "" {
			checkInTime = "09:00" // Default fallback
		}
		checkOutTime := calculateCheckoutTime(checkInTime, preTripLounge.PricingType)

		hold := &models.LoungeCapacityHold{
			ID:            uuid.New(),
			LoungeID:      loungeID,
			IntentID:      intent.ID,
			Date:          loungeDate,
			TimeSlotStart: checkInTime,
			TimeSlotEnd:   checkOutTime,
			GuestsCount:   preTripLounge.GuestCount,
			HeldUntil:     expiresAt,
			Status:        "held",
			CreatedAt:     time.Now(),
		}
		if err := s.intentRepo.CreateLoungeCapacityHold(hold); err != nil {
			s.logger.WithError(err).Warn("Failed to create pre-trip lounge hold")
		}
	}

	if postTripLounge != nil {
		postLoungeFare = postTripLounge.TotalPrice
		// Create lounge capacity hold using actual lounge date/time
		expiresAt := time.Now().Add(s.config.IntentTTL)
		loungeID, _ := uuid.Parse(postTripLounge.LoungeID)

		loungeDate := parseLoungeDate(postTripLounge.Date)
		checkInTime := postTripLounge.CheckInTime
		if checkInTime == "" {
			checkInTime = "09:00" // Default fallback
		}
		checkOutTime := calculateCheckoutTime(checkInTime, postTripLounge.PricingType)

		hold := &models.LoungeCapacityHold{
			ID:            uuid.New(),
			LoungeID:      loungeID,
			IntentID:      intent.ID,
			Date:          loungeDate,
			TimeSlotStart: checkInTime,
			TimeSlotEnd:   checkOutTime,
			GuestsCount:   postTripLounge.GuestCount,
			HeldUntil:     expiresAt,
			Status:        "held",
			CreatedAt:     time.Now(),
		}
		if err := s.intentRepo.CreateLoungeCapacityHold(hold); err != nil {
			s.logger.WithError(err).Warn("Failed to create post-trip lounge hold")
		}
	}

	// 3. Update intent with lounge data
	newTotal := intent.BusFare + preLoungeFare + postLoungeFare
	newExpiresAt := time.Now().Add(s.config.IntentTTL) // Extend the hold timer

	s.logger.WithFields(logrus.Fields{
		"intent_id":        intent.ID,
		"has_pre_lounge":   preTripLounge != nil,
		"has_post_lounge":  postTripLounge != nil,
		"pre_lounge_fare":  preLoungeFare,
		"post_lounge_fare": postLoungeFare,
		"new_total":        newTotal,
	}).Info("AddLoungeToIntent: Saving lounge data to intent")

	err = s.intentRepo.AddLoungeToIntent(
		intent.ID,
		preTripLounge,
		postTripLounge,
		preLoungeFare,
		postLoungeFare,
		newTotal,
		newExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update intent with lounges: %w", err)
	}

	s.logger.WithField("intent_id", intent.ID).Info("AddLoungeToIntent: Lounge data saved successfully")

	// 4. Extend seat holds to match new expiration
	if err := s.intentRepo.ExtendSeatHolds(intent.ID, newExpiresAt); err != nil {
		s.logger.WithError(err).Warn("Failed to extend seat holds")
	}

	// 5. Fetch updated intent and verify lounge data was saved
	updatedIntent, err := s.intentRepo.GetIntentByID(intent.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated intent: %w", err)
	}

	// Log verification of saved data
	verifyFields := logrus.Fields{
		"intent_id":              updatedIntent.ID,
		"intent_type":            updatedIntent.IntentType,
		"has_pre_lounge_intent":  updatedIntent.PreTripLoungeIntent != nil,
		"has_post_lounge_intent": updatedIntent.PostTripLoungeIntent != nil,
		"pre_lounge_fare":        updatedIntent.PreLoungeFare,
		"post_lounge_fare":       updatedIntent.PostLoungeFare,
		"total_amount":           updatedIntent.TotalAmount,
	}
	// Add lounge IDs if present for confirmation
	if updatedIntent.PreTripLoungeIntent != nil {
		verifyFields["pre_lounge_id"] = updatedIntent.PreTripLoungeIntent.LoungeID
		verifyFields["pre_lounge_name"] = updatedIntent.PreTripLoungeIntent.LoungeName
	}
	if updatedIntent.PostTripLoungeIntent != nil {
		verifyFields["post_lounge_id"] = updatedIntent.PostTripLoungeIntent.LoungeID
	}
	s.logger.WithFields(verifyFields).Info("AddLoungeToIntent: Verified saved intent data")

	return s.buildIntentResponse(updatedIntent), nil
}

// ============================================================================
// CANCEL INTENT
// ============================================================================

// CancelIntent cancels a booking intent and releases all holds
func (s *BookingOrchestratorService) CancelIntent(intentID uuid.UUID, userID uuid.UUID) error {
	intent, err := s.intentRepo.GetIntentByID(intentID)
	if err != nil {
		return fmt.Errorf("failed to get intent: %w", err)
	}
	if intent == nil {
		return fmt.Errorf("intent not found")
	}

	// Verify ownership
	if intent.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	// Check if can cancel
	if intent.Status == models.IntentStatusConfirmed {
		return fmt.Errorf("cannot cancel confirmed intent, use booking cancellation instead")
	}
	if intent.Status == models.IntentStatusExpired || intent.Status == models.IntentStatusCancelled {
		return nil // Already cancelled/expired
	}

	// Release all holds
	s.rollbackHolds(intentID)

	// Mark as cancelled
	return s.intentRepo.UpdateIntentCancelled(intentID)
}

// ============================================================================
// HELPER METHODS
// ============================================================================

func (s *BookingOrchestratorService) rollbackHolds(intentID uuid.UUID) {
	if err := s.intentRepo.ReleaseSeatHoldsForIntent(intentID); err != nil {
		s.logger.WithError(err).WithField("intent_id", intentID).Error("Failed to release seat holds")
	}
	if err := s.intentRepo.ReleaseLoungeHoldsForIntent(intentID); err != nil {
		s.logger.WithError(err).WithField("intent_id", intentID).Error("Failed to release lounge holds")
	}
}

func (s *BookingOrchestratorService) buildIntentResponse(intent *models.BookingIntent) *models.BookingIntentResponse {
	ttl := int(time.Until(intent.ExpiresAt).Seconds())
	if ttl < 0 {
		ttl = 0
	}

	return &models.BookingIntentResponse{
		IntentID: intent.ID,
		Status:   string(intent.Status),
		PriceBreakdown: models.PriceBreakdown{
			BusFare:        intent.BusFare,
			PreLoungeFare:  intent.PreLoungeFare,
			PostLoungeFare: intent.PostLoungeFare,
			Total:          intent.TotalAmount,
			Currency:       intent.Currency,
		},
		ExpiresAt:                 intent.ExpiresAt,
		TTLSeconds:                ttl,
		SeatAvailabilityChecked:   intent.BusIntent != nil,
		LoungeAvailabilityChecked: intent.PreTripLoungeIntent != nil || intent.PostTripLoungeIntent != nil,
	}
}

func (s *BookingOrchestratorService) buildConfirmResponse(intent *models.BookingIntent) *models.ConfirmBookingResponse {
	response := &models.ConfirmBookingResponse{
		TotalPaid: intent.TotalAmount,
		Currency:  intent.Currency,
	}

	s.logger.WithFields(logrus.Fields{
		"intent_id":              intent.ID,
		"bus_booking_id":         intent.BusBookingID,
		"pre_lounge_booking_id":  intent.PreLoungeBookingID,
		"post_lounge_booking_id": intent.PostLoungeBookingID,
	}).Info("Building confirm response with booking IDs")

	// Get bus booking details
	if intent.BusBookingID != nil {
		busBooking, err := s.appBookingRepo.GetBusBookingByID(intent.BusBookingID.String())
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"error":          err.Error(),
				"bus_booking_id": intent.BusBookingID,
			}).Error("Failed to get bus booking for confirm response")
		} else if busBooking != nil {
			// Get master booking for reference
			masterBooking, masterErr := s.appBookingRepo.GetBookingByID(busBooking.BookingID)
			if masterErr != nil {
				s.logger.WithError(masterErr).Error("Failed to get master booking")
			} else if masterBooking != nil {
				response.BusBooking = &models.ConfirmedBusBooking{
					ID:          uuid.MustParse(busBooking.ID),
					Reference:   masterBooking.BookingReference,
					TotalAmount: busBooking.TotalFare,
				}
				if busBooking.QRCodeData != nil {
					response.BusBooking.QRCode = *busBooking.QRCodeData
				}
				response.MasterReference = masterBooking.BookingReference
				s.logger.WithFields(logrus.Fields{
					"bus_ref": masterBooking.BookingReference,
					"has_qr":  busBooking.QRCodeData != nil,
				}).Info("Bus booking added to confirm response")
			}
		}
	}

	// Get pre-lounge booking details
	if intent.PreLoungeBookingID != nil {
		s.logger.WithField("lounge_booking_id", intent.PreLoungeBookingID.String()).Info("Fetching pre-lounge booking details")
		loungeBooking, err := s.loungeBookingRepo.GetLoungeBookingByID(*intent.PreLoungeBookingID)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"error":             fmt.Sprintf("%v", err),
				"lounge_booking_id": intent.PreLoungeBookingID.String(),
			}).Error("Failed to get pre-lounge booking for confirm response")
		} else if loungeBooking == nil {
			s.logger.WithField("lounge_booking_id", intent.PreLoungeBookingID.String()).Warn("Pre-lounge booking not found in database despite having ID")
		} else {
			response.PreLoungeBooking = &models.ConfirmedLoungeBooking{
				ID:        loungeBooking.ID,
				Reference: loungeBooking.BookingReference,
			}
			if loungeBooking.QRCodeData != nil {
				response.PreLoungeBooking.QRCode = loungeBooking.QRCodeData
			}
			s.logger.WithFields(logrus.Fields{
				"pre_lounge_ref": loungeBooking.BookingReference,
				"has_qr_code":    loungeBooking.QRCodeData != nil,
			}).Info("Pre-lounge booking added to confirm response")
		}
	}

	// Get post-lounge booking details
	if intent.PostLoungeBookingID != nil {
		s.logger.WithField("lounge_booking_id", intent.PostLoungeBookingID.String()).Info("Fetching post-lounge booking details")
		loungeBooking, err := s.loungeBookingRepo.GetLoungeBookingByID(*intent.PostLoungeBookingID)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"error":             fmt.Sprintf("%v", err),
				"lounge_booking_id": intent.PostLoungeBookingID.String(),
			}).Error("Failed to get post-lounge booking for confirm response")
		} else if loungeBooking == nil {
			s.logger.WithField("lounge_booking_id", intent.PostLoungeBookingID.String()).Warn("Post-lounge booking not found in database despite having ID")
		} else {
			response.PostLoungeBooking = &models.ConfirmedLoungeBooking{
				ID:        loungeBooking.ID,
				Reference: loungeBooking.BookingReference,
			}
			if loungeBooking.QRCodeData != nil {
				response.PostLoungeBooking.QRCode = loungeBooking.QRCodeData
			}
			s.logger.WithFields(logrus.Fields{
				"post_lounge_ref": loungeBooking.BookingReference,
				"has_qr_code":     loungeBooking.QRCodeData != nil,
			}).Info("Post-lounge booking added to confirm response")
		}
	}

	s.logger.WithFields(logrus.Fields{
		"has_bus_booking":  response.BusBooking != nil,
		"has_pre_lounge":   response.PreLoungeBooking != nil,
		"has_post_lounge":  response.PostLoungeBooking != nil,
		"master_reference": response.MasterReference,
	}).Info("Confirm response built successfully")

	return response
}

func (s *BookingOrchestratorService) buildPartialAvailabilityError(
	unavailableSeats []string,
	unavailablePreLounge *models.UnavailableReason,
	unavailablePostLounge *models.UnavailableReason,
) *models.PartialAvailabilityError {
	err := &models.PartialAvailabilityError{
		Message:     "Some items are no longer available",
		Available:   models.AvailabilityStatus{},
		Unavailable: models.UnavailableItems{},
	}

	if len(unavailableSeats) > 0 {
		err.Unavailable.Bus = &models.UnavailableReason{
			Reason:     "seats_taken",
			Details:    fmt.Sprintf("%d seat(s) are no longer available", len(unavailableSeats)),
			TakenSeats: unavailableSeats,
		}
	}

	if unavailablePreLounge != nil {
		err.Unavailable.PreLounge = unavailablePreLounge
	}

	if unavailablePostLounge != nil {
		err.Unavailable.PostLounge = unavailablePostLounge
	}

	return err
}

// GetIntentsByUser retrieves all intents for a user with pagination
func (s *BookingOrchestratorService) GetIntentsByUser(userID uuid.UUID, limit, offset int) ([]*models.BookingIntent, error) {
	return s.intentRepo.GetIntentsByUserID(userID, limit, offset)
}
