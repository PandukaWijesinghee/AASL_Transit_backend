package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// TripGeneratorService handles automatic generation of scheduled trips from schedules
type TripGeneratorService struct {
	scheduleRepo      *database.TripScheduleRepository
	scheduledTripRepo *database.ScheduledTripRepository
	busRepo           *database.BusRepository
	settingsRepo      *database.SystemSettingRepository
}

// NewTripGeneratorService creates a new TripGeneratorService
func NewTripGeneratorService(
	scheduleRepo *database.TripScheduleRepository,
	scheduledTripRepo *database.ScheduledTripRepository,
	busRepo *database.BusRepository,
	settingsRepo *database.SystemSettingRepository,
) *TripGeneratorService {
	return &TripGeneratorService{
		scheduleRepo:      scheduleRepo,
		scheduledTripRepo: scheduledTripRepo,
		busRepo:           busRepo,
		settingsRepo:      settingsRepo,
	}
}

// GenerateTripsForSchedule generates scheduled trips for a given schedule and date range
func (s *TripGeneratorService) GenerateTripsForSchedule(schedule *models.TripSchedule, startDate, endDate time.Time) (int, error) {
	generated := 0
	currentDate := startDate

	for currentDate.Before(endDate) || currentDate.Equal(endDate) {
		// Check if schedule is valid for this date
		if schedule.IsValidForDate(currentDate) {
			// Check if trip already exists for this date
			existing, err := s.scheduledTripRepo.GetByScheduleAndDate(schedule.ID, currentDate)
			if err == nil && existing != nil {
				// Trip already exists, skip
				currentDate = currentDate.AddDate(0, 0, 1)
				continue
			}

			// Get total seats from bus (if assigned)
			totalSeats := 50 // Default
			if schedule.BusID != nil {
				bus, err := s.busRepo.GetByID(*schedule.BusID)
				if err == nil {
					totalSeats = bus.TotalSeats
				}
			}

			// Calculate max bookable seats
			maxBookableSeats := totalSeats
			if schedule.MaxBookableSeats != nil && *schedule.MaxBookableSeats < totalSeats {
				maxBookableSeats = *schedule.MaxBookableSeats
			}

			// Determine booking advance hours (use schedule's or default)
			bookingAdvanceHours := 72 // system default
			if schedule.BookingAdvanceHours != nil {
				bookingAdvanceHours = *schedule.BookingAdvanceHours
			}

			// Calculate assignment deadline (e.g., 2 hours before departure)
			// TODO: Get assignment_deadline_hours from system settings
			assignmentDeadlineHours := 2
			departureDateTime := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 0, 0, 0, 0, currentDate.Location())
			// Parse departure time and add to date
			if t, err := time.Parse("15:04", schedule.DepartureTime); err == nil {
				departureDateTime = time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), t.Hour(), t.Minute(), 0, 0, currentDate.Location())
			}
			assignmentDeadline := departureDateTime.Add(-time.Duration(assignmentDeadlineHours) * time.Hour)

			// Create scheduled trip
			scheduleID := schedule.ID
			permitID := ""
			if schedule.PermitID != nil {
				permitID = *schedule.PermitID
			}
			trip := &models.ScheduledTrip{
				ID:                   uuid.New().String(),
				TripScheduleID:       &scheduleID,
				CustomRouteID:        schedule.CustomRouteID,
				PermitID:             permitID,
				BusID:                schedule.BusID,
				TripDate:             currentDate,
				DepartureTime:        schedule.DepartureTime,
				EstimatedArrivalTime: schedule.EstimatedArrivalTime,
				AssignedDriverID:     schedule.DefaultDriverID,
				AssignedConductorID:  schedule.DefaultConductorID,
				IsBookable:           schedule.IsBookable,
				TotalSeats:           totalSeats,
				AvailableSeats:       maxBookableSeats,
				BookedSeats:          0,
				BaseFare:             schedule.BaseFare,
				BookingAdvanceHours:  bookingAdvanceHours,
				AssignmentDeadline:   &assignmentDeadline,
				Status:               models.ScheduledTripStatusScheduled,
				SelectedStopIDs:      schedule.SelectedStopIDs,
			}

			if err := s.scheduledTripRepo.Create(trip); err != nil {
				// Log error but continue with other dates
				fmt.Printf("Failed to create trip for date %s: %v\n", currentDate.Format("2006-01-02"), err)
			} else {
				generated++
			}
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return generated, nil
}

// GenerateTripsForNewSchedule generates trips for a newly created schedule
// Uses trip_generation_days_ahead from system_settings (default: 7 days)
func (s *TripGeneratorService) GenerateTripsForNewSchedule(schedule *models.TripSchedule) (int, error) {
	startDate := time.Now()

	// Start from valid_from if it's in the future
	if schedule.ValidFrom.After(startDate) {
		startDate = schedule.ValidFrom
	}

	// Get days ahead from system settings (default: 7)
	daysAhead := s.settingsRepo.GetIntValue("trip_generation_days_ahead", 7)

	// Generate for configured days ahead
	endDate := startDate.AddDate(0, 0, daysAhead)

	// Don't exceed valid_until
	if schedule.ValidUntil != nil && endDate.After(*schedule.ValidUntil) {
		endDate = *schedule.ValidUntil
	}

	return s.GenerateTripsForSchedule(schedule, startDate, endDate)
}

// GenerateFutureTrips generates trips for all active timetables (maintains 7 occurrences ahead)
// This is called by the cron job at 1-2 AM daily
func (s *TripGeneratorService) GenerateFutureTrips() (int, error) {
	// Get all active timetables
	timetables, err := s.scheduleRepo.GetAllActiveTimetables()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch active timetables: %w", err)
	}

	totalGenerated := 0

	for _, timetable := range timetables {
		// Get next 7 occurrences for this timetable
		nextDates := timetable.GetNextOccurrences(7)

		for _, date := range nextDates {
			// Check if trip already exists for this date
			existing, err := s.scheduledTripRepo.GetByScheduleAndDate(timetable.ID, date)
			if err == nil && existing != nil {
				// Trip already exists, skip
				continue
			}

			// Get total seats (default to permit seating capacity)
			totalSeats := 50 // Default
			maxBookableSeats := totalSeats
			if timetable.MaxBookableSeats != nil {
				maxBookableSeats = *timetable.MaxBookableSeats
				totalSeats = *timetable.MaxBookableSeats
			}

			// Determine booking advance hours
			bookingAdvanceHours := 72 // system default
			if timetable.BookingAdvanceHours != nil {
				bookingAdvanceHours = *timetable.BookingAdvanceHours
			}

			// Calculate assignment deadline (2 hours before departure)
			assignmentDeadlineHours := 2
			departureDateTime := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
			// Parse departure time
			if t, err := time.Parse("15:04", timetable.DepartureTime); err == nil {
				departureDateTime = time.Date(date.Year(), date.Month(), date.Day(), t.Hour(), t.Minute(), 0, 0, date.Location())
			}
			assignmentDeadline := departureDateTime.Add(-time.Duration(assignmentDeadlineHours) * time.Hour)

			// Create scheduled trip
			scheduleID := timetable.ID
			permitID := ""
			if timetable.PermitID != nil {
				permitID = *timetable.PermitID
			}
			trip := &models.ScheduledTrip{
				ID:                   uuid.New().String(),
				TripScheduleID:       &scheduleID,
				CustomRouteID:        timetable.CustomRouteID,
				PermitID:             permitID,
				BusID:                timetable.BusID,
				TripDate:             date,
				DepartureTime:        timetable.DepartureTime,
				EstimatedArrivalTime: timetable.EstimatedArrivalTime,
				AssignedDriverID:     timetable.DefaultDriverID,
				AssignedConductorID:  timetable.DefaultConductorID,
				IsBookable:           timetable.IsBookable,
				TotalSeats:           totalSeats,
				AvailableSeats:       maxBookableSeats,
				BookedSeats:          0,
				BaseFare:             timetable.BaseFare,
				BookingAdvanceHours:  bookingAdvanceHours,
				AssignmentDeadline:   &assignmentDeadline,
				Status:               models.ScheduledTripStatusScheduled,
				SelectedStopIDs:      timetable.SelectedStopIDs,
			}

			if err := s.scheduledTripRepo.Create(trip); err != nil {
				fmt.Printf("Failed to create trip for timetable %s on %s: %v\n", timetable.ID, date.Format("2006-01-02"), err)
				continue
			}

			totalGenerated++
		}
	}

	return totalGenerated, nil
}

// RegenerateTripsForSchedule regenerates trips for a schedule (useful after updates)
// Regenerates only future trips that haven't started yet
// Uses trip_generation_days_ahead from system_settings (default: 7 days)
func (s *TripGeneratorService) RegenerateTripsForSchedule(schedule *models.TripSchedule) (int, error) {
	startDate := time.Now()

	// Start from valid_from if it's in the future
	if schedule.ValidFrom.After(startDate) {
		startDate = schedule.ValidFrom
	}

	// Get days ahead from system settings (default: 7)
	daysAhead := s.settingsRepo.GetIntValue("trip_generation_days_ahead", 7)

	// Generate for configured days ahead
	endDate := startDate.AddDate(0, 0, daysAhead)

	// Don't exceed valid_until
	if schedule.ValidUntil != nil && endDate.After(*schedule.ValidUntil) {
		endDate = *schedule.ValidUntil
	}

	// Note: This will skip trips that already exist (handled in GenerateTripsForSchedule)
	return s.GenerateTripsForSchedule(schedule, startDate, endDate)
}

// CleanupOldTrips removes completed trips older than specified days
func (s *TripGeneratorService) CleanupOldTrips(daysToKeep int) error {
	// This would delete old completed trips
	// Implementation depends on if you want to keep historical data
	// For now, we'll keep all data for reporting
	return nil
}

// FillMissingTrips scans for any gaps in scheduled trips and fills them
// Useful for recovering from downtime or errors
// Uses trip_generation_days_ahead from system_settings for range
func (s *TripGeneratorService) FillMissingTrips() (int, error) {
	startDate := time.Now()

	// Get days ahead from system settings (default: 7)
	daysAhead := s.settingsRepo.GetIntValue("trip_generation_days_ahead", 7)
	endDate := startDate.AddDate(0, 0, daysAhead)

	schedules, err := s.scheduleRepo.GetActiveSchedulesForDate(startDate)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch active schedules: %w", err)
	}

	totalGenerated := 0

	for _, schedule := range schedules {
		// Respect schedule's valid_from and valid_until
		scheduleStartDate := startDate
		if schedule.ValidFrom.After(startDate) {
			scheduleStartDate = schedule.ValidFrom
		}

		scheduleEndDate := endDate
		if schedule.ValidUntil != nil && endDate.After(*schedule.ValidUntil) {
			scheduleEndDate = *schedule.ValidUntil
		}

		generated, err := s.GenerateTripsForSchedule(&schedule, scheduleStartDate, scheduleEndDate)
		if err != nil {
			fmt.Printf("Error filling missing trips for schedule %s: %v\n", schedule.ID, err)
			continue
		}

		totalGenerated += generated
	}

	return totalGenerated, nil
}

// GetGenerationStats returns statistics about trip generation
type GenerationStats struct {
	TotalSchedules    int       `json:"total_schedules"`
	ActiveSchedules   int       `json:"active_schedules"`
	TripsGenerated    int       `json:"trips_generated"`
	NextRunDate       time.Time `json:"next_run_date"`
	LastRunDate       time.Time `json:"last_run_date"`
	AverageTripPerDay float64   `json:"average_trips_per_day"`
}

// GetStats returns generation statistics
func (s *TripGeneratorService) GetStats() (*GenerationStats, error) {
	// Get active schedules
	today := time.Now()
	schedules, err := s.scheduleRepo.GetActiveSchedulesForDate(today)
	if err != nil {
		return nil, err
	}

	// Get trips for next 7 days
	startDate := time.Now()
	endDate := startDate.AddDate(0, 0, 7)
	trips, err := s.scheduledTripRepo.GetByDateRange(startDate, endDate)
	if err != nil {
		return nil, err
	}

	avgPerDay := float64(len(trips)) / 7.0

	stats := &GenerationStats{
		ActiveSchedules:   len(schedules),
		TripsGenerated:    len(trips),
		AverageTripPerDay: avgPerDay,
		LastRunDate:       time.Now(), // Would be stored in DB in production
		NextRunDate:       time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour),
	}

	return stats, nil
}
