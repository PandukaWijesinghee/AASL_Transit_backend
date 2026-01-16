package services

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// IntentExpirationService handles background expiration of booking intents
type IntentExpirationService struct {
	intentRepo *database.BookingIntentRepository
	logger     *logrus.Logger
	stopCh     chan struct{}
	interval   time.Duration
}

// NewIntentExpirationService creates a new intent expiration service
func NewIntentExpirationService(
	intentRepo *database.BookingIntentRepository,
	logger *logrus.Logger,
) *IntentExpirationService {
	return &IntentExpirationService{
		intentRepo: intentRepo,
		logger:     logger,
		stopCh:     make(chan struct{}),
		interval:   1 * time.Minute, // Check every minute
	}
}

// Start begins the background expiration job
func (s *IntentExpirationService) Start() {
	s.logger.Info("🕐 Starting Intent Expiration Service (checking every minute)")
	go s.run()
}

// Stop stops the background expiration job
func (s *IntentExpirationService) Stop() {
	s.logger.Info("🛑 Stopping Intent Expiration Service")
	close(s.stopCh)
}

func (s *IntentExpirationService) run() {
	// Run immediately on start
	s.processExpiredIntents()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processExpiredIntents()
		case <-s.stopCh:
			s.logger.Info("Intent Expiration Service stopped")
			return
		}
	}
}

// processExpiredIntents finds and expires held intents past their TTL
func (s *IntentExpirationService) processExpiredIntents() {
	// 1. Find expired held intents (process up to 100 at a time)
	expiredIntents, err := s.intentRepo.GetExpiredHeldIntents(100)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get expired intents")
		return
	}

	if len(expiredIntents) == 0 {
		return // Nothing to expire
	}

	s.logger.WithField("count", len(expiredIntents)).Info("Processing expired intents")

	for _, intent := range expiredIntents {
		err := s.expireIntent(intent)
		if err != nil {
			s.logger.WithError(err).WithField("intent_id", intent.ID).Error("Failed to expire intent")
		} else {
			s.logger.WithField("intent_id", intent.ID).Info("Intent expired and holds released")
		}
	}

	// 2. Release any orphan seat holds (safety cleanup)
	orphanReleased, err := s.intentRepo.ReleaseOrphanSeatHolds()
	if err != nil {
		s.logger.WithError(err).Error("Failed to release orphan seat holds")
	} else if orphanReleased > 0 {
		s.logger.WithField("count", orphanReleased).Warn("Released orphan seat holds")
	}

	// 3. Release expired seat holds based on held_until timestamp
	expiredSeats, err := s.intentRepo.ReleaseExpiredSeatHolds()
	if err != nil {
		s.logger.WithError(err).Error("Failed to release expired seat holds")
	} else if expiredSeats > 0 {
		s.logger.WithField("count", expiredSeats).Info("Released expired seat holds")
	}
}

// expireIntent marks an intent as expired and releases all its holds
func (s *IntentExpirationService) expireIntent(intent *models.BookingIntent) error {
	return s.intentRepo.ExpireIntentAndReleaseHolds(intent.ID)
}

// RunOnce runs a single expiration cycle (useful for testing or manual trigger)
func (s *IntentExpirationService) RunOnce() {
	s.processExpiredIntents()
}

// GetStats returns statistics about expired intents (for admin dashboard)
func (s *IntentExpirationService) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Could add more stats like:
	// - Total expired today
	// - Currently held intents
	// - Average hold duration

	return stats
}
