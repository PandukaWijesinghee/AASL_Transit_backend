package services

import (
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// CronService manages scheduled background jobs
type CronService struct {
	cron             *cron.Cron
	tripGeneratorSvc *TripGeneratorService
}

// NewCronService creates a new CronService
func NewCronService(tripGeneratorSvc *TripGeneratorService) *CronService {
	// Create cron with seconds precision (optional)
	c := cron.New(cron.WithSeconds())

	return &CronService{
		cron:             c,
		tripGeneratorSvc: tripGeneratorSvc,
	}
}

// Start starts all cron jobs
func (s *CronService) Start() error {
	log.Println("Starting cron service...")

	// Job 1: Generate future trips daily at 2 AM
	// Cron format: second minute hour day month weekday
	// "0 0 2 * * *" = At 2:00 AM every day
	_, err := s.cron.AddFunc("0 0 2 * * *", s.generateFutureTripsJob)
	if err != nil {
		return fmt.Errorf("failed to schedule future trips job: %w", err)
	}
	log.Println("✓ Scheduled: Generate future trips (Daily at 2:00 AM)")

	// Job 2: Fill missing trips daily at 3 AM (backup/recovery)
	// "0 0 3 * * *" = At 3:00 AM every day
	_, err = s.cron.AddFunc("0 0 3 * * *", s.fillMissingTripsJob)
	if err != nil {
		return fmt.Errorf("failed to schedule fill missing trips job: %w", err)
	}
	log.Println("✓ Scheduled: Fill missing trips (Daily at 3:00 AM)")

	// Job 3: Cleanup old trips weekly on Sunday at 4 AM
	// "0 0 4 * * 0" = At 4:00 AM every Sunday
	_, err = s.cron.AddFunc("0 0 4 * * 0", s.cleanupOldTripsJob)
	if err != nil {
		return fmt.Errorf("failed to schedule cleanup job: %w", err)
	}
	log.Println("✓ Scheduled: Cleanup old trips (Sundays at 4:00 AM)")

	// Start the cron scheduler
	s.cron.Start()
	log.Println("✓ Cron service started successfully")

	return nil
}

// Stop stops all cron jobs
func (s *CronService) Stop() {
	log.Println("Stopping cron service...")
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("✓ Cron service stopped")
}

// generateFutureTripsJob generates trips for days 14-30
func (s *CronService) generateFutureTripsJob() {
	log.Println("[CRON] Starting future trips generation job...")
	startTime := time.Now()

	tripsGenerated, err := s.tripGeneratorSvc.GenerateFutureTrips()
	if err != nil {
		log.Printf("[CRON ERROR] Failed to generate future trips: %v\n", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("[CRON] ✓ Generated %d trips in %v\n", tripsGenerated, duration)
}

// fillMissingTripsJob fills any gaps in scheduled trips (recovery)
func (s *CronService) fillMissingTripsJob() {
	log.Println("[CRON] Starting fill missing trips job...")
	startTime := time.Now()

	tripsGenerated, err := s.tripGeneratorSvc.FillMissingTrips()
	if err != nil {
		log.Printf("[CRON ERROR] Failed to fill missing trips: %v\n", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("[CRON] ✓ Filled %d missing trips in %v\n", tripsGenerated, duration)
}

// cleanupOldTripsJob cleans up old completed trips (keeps last 90 days)
func (s *CronService) cleanupOldTripsJob() {
	log.Println("[CRON] Starting cleanup old trips job...")
	startTime := time.Now()

	err := s.tripGeneratorSvc.CleanupOldTrips(90)
	if err != nil {
		log.Printf("[CRON ERROR] Failed to cleanup old trips: %v\n", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("[CRON] ✓ Cleaned up old trips in %v\n", duration)
}

// RunGenerateFutureTripsNow runs the future trips generation job immediately (for testing)
func (s *CronService) RunGenerateFutureTripsNow() error {
	log.Println("[MANUAL] Running future trips generation now...")
	s.generateFutureTripsJob()
	return nil
}

// RunFillMissingTripsNow runs the fill missing trips job immediately (for testing)
func (s *CronService) RunFillMissingTripsNow() error {
	log.Println("[MANUAL] Running fill missing trips now...")
	s.fillMissingTripsJob()
	return nil
}

// GetJobStatus returns the status of scheduled jobs
func (s *CronService) GetJobStatus() map[string]interface{} {
	entries := s.cron.Entries()

	jobs := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		jobs = append(jobs, map[string]interface{}{
			"id":        entry.ID,
			"next_run":  entry.Next,
			"prev_run":  entry.Prev,
		})
	}

	return map[string]interface{}{
		"running":    len(entries) > 0,
		"job_count":  len(entries),
		"jobs":       jobs,
	}
}

// NOTE: To use this service, you need to:
// 1. Install the cron library: go get github.com/robfig/cron/v3
// 2. Initialize in main.go:
//
//    cronService := services.NewCronService(tripGeneratorSvc)
//    if err := cronService.Start(); err != nil {
//        log.Fatal("Failed to start cron service:", err)
//    }
//    defer cronService.Stop()
//
// 3. Optional: Add admin endpoint to trigger jobs manually:
//
//    router.POST("/admin/cron/generate-trips", func(c *gin.Context) {
//        cronService.RunGenerateFutureTripsNow()
//        c.JSON(200, gin.H{"message": "Job triggered"})
//    })
