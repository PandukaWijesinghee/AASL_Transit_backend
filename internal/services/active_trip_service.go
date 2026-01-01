package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// ActiveTripService interface defines the service methods
type ActiveTripService interface {
	StartTrip(input *StartTripInput) (*StartTripResult, error)
	UpdateLocation(input *UpdateLocationInput) error
	EndTrip(input *EndTripInput) (*EndTripResult, error)
	GetActiveTrip(activeTripID string) (*models.ActiveTrip, error)
	GetActiveTripByScheduledTrip(scheduledTripID string) (*models.ActiveTrip, error)
	GetMyActiveTrip(staffID string) (*models.ActiveTrip, error)
	UpdatePassengerCount(activeTripID string, staffID string, count int) error
	GetPassengersByScheduledTripID(scheduledTripID string) ([]models.PassengerInfo, error)
	UpdatePassengerStatus(bookingID string, status string) error
	BoardPassenger(bookingID string) error
	VerifyPassengerBooking(reference string, scheduledTripID string) (*models.MasterBooking, error)
}

// activeTripService implements ActiveTripService
type activeTripService struct {
	activeTripRepo    *database.ActiveTripRepository
	scheduledTripRepo *database.ScheduledTripRepository
	staffRepo         *database.BusStaffRepository
	busRepo           *database.BusRepository
	permitRepo        *database.RoutePermitRepository
	appBookingRepo    *database.AppBookingRepository
}

// NewActiveTripService creates a new ActiveTripService
func NewActiveTripService(
	activeTripRepo *database.ActiveTripRepository,
	scheduledTripRepo *database.ScheduledTripRepository,
	staffRepo *database.BusStaffRepository,
	busRepo *database.BusRepository,
	permitRepo *database.RoutePermitRepository,
	appBookingRepo *database.AppBookingRepository,
) ActiveTripService {
	return &activeTripService{
		activeTripRepo:    activeTripRepo,
		scheduledTripRepo: scheduledTripRepo,
		staffRepo:         staffRepo,
		busRepo:           busRepo,
		permitRepo:        permitRepo,
		appBookingRepo:    appBookingRepo,
	}
}

// StartTripInput contains the data needed to start a trip
type StartTripInput struct {
	ScheduledTripID  string  `json:"scheduled_trip_id"`
	StaffID          string  `json:"staff_id"` // The staff member starting the trip
	InitialLatitude  float64 `json:"initial_latitude"`
	InitialLongitude float64 `json:"initial_longitude"`
}

// StartTripResult contains the result of starting a trip
type StartTripResult struct {
	ActiveTrip      *models.ActiveTrip `json:"active_trip"`
	Message         string             `json:"message"`
	ScheduledTripID string             `json:"scheduled_trip_id"`
}

// StartTrip starts a scheduled trip - creates active_trip record and updates scheduled_trip status
func (s *activeTripService) StartTrip(input *StartTripInput) (*StartTripResult, error) {
	log.Printf("[StartTrip] === START TRIP DEBUG v2 ===")
	log.Printf("[StartTrip] Input: ScheduledTripID=%s, StaffID=%s, Lat=%f, Lon=%f",
		input.ScheduledTripID, input.StaffID, input.InitialLatitude, input.InitialLongitude)

	// 1. Get the scheduled trip
	scheduledTrip, err := s.scheduledTripRepo.GetByID(input.ScheduledTripID)
	if err != nil {
		log.Printf("[StartTrip] ERROR: Failed to get scheduled trip: %v", err)
		return nil, errors.New("scheduled trip not found")
	}
	log.Printf("[StartTrip] Got scheduled trip: ID=%s, Status=%s", scheduledTrip.ID, scheduledTrip.Status)

	// 2. Validate the scheduled trip can be started
	if scheduledTrip.Status != "scheduled" && scheduledTrip.Status != "confirmed" {
		log.Printf("[StartTrip] ERROR: Invalid status: %s", scheduledTrip.Status)
		return nil, errors.New("trip cannot be started - current status: " + string(scheduledTrip.Status))
	}

	// Log assigned staff - check for nil before dereferencing
	log.Printf("[StartTrip] AssignedDriverID is nil: %v", scheduledTrip.AssignedDriverID == nil)
	log.Printf("[StartTrip] AssignedConductorID is nil: %v", scheduledTrip.AssignedConductorID == nil)
	if scheduledTrip.AssignedDriverID != nil {
		log.Printf("[StartTrip] AssignedDriverID value: %s", *scheduledTrip.AssignedDriverID)
	}
	if scheduledTrip.AssignedConductorID != nil {
		log.Printf("[StartTrip] AssignedConductorID value: %s", *scheduledTrip.AssignedConductorID)
	}

	// 3. Verify the staff is assigned to this trip
	isDriver := scheduledTrip.AssignedDriverID != nil && *scheduledTrip.AssignedDriverID == input.StaffID
	isConductor := scheduledTrip.AssignedConductorID != nil && *scheduledTrip.AssignedConductorID == input.StaffID
	log.Printf("[StartTrip] Staff check: isDriver=%v, isConductor=%v", isDriver, isConductor)
	if !isDriver && !isConductor {
		log.Printf("[StartTrip] ERROR: Staff not assigned to trip")
		return nil, errors.New("you are not assigned to this trip")
	}

	// 4. Check if an active trip already exists for this scheduled trip
	log.Printf("[StartTrip] Checking for existing active trip...")
	existingActiveTrip, err := s.activeTripRepo.GetByScheduledTripID(input.ScheduledTripID)
	if err == nil && existingActiveTrip != nil {
		log.Printf("[StartTrip] Found existing active trip: ID=%s, Status=%s", existingActiveTrip.ID, existingActiveTrip.Status)
		// Active trip already exists
		if existingActiveTrip.IsActive() {
			log.Printf("[StartTrip] Trip already active, returning existing")
			return &StartTripResult{
				ActiveTrip:      existingActiveTrip,
				Message:         "Trip already started",
				ScheduledTripID: input.ScheduledTripID,
			}, nil
		}
		// Trip was completed/cancelled, can't restart
		log.Printf("[StartTrip] ERROR: Trip already completed/cancelled")
		return nil, errors.New("trip has already been completed or cancelled")
	}
	log.Printf("[StartTrip] No existing active trip found (this is expected for new trips)")

	// 5. Get bus and permit info from scheduled trip
	log.Printf("[StartTrip] PermitID is nil: %v", scheduledTrip.PermitID == nil)
	if scheduledTrip.PermitID == nil {
		log.Printf("[StartTrip] ERROR: No permit assigned")
		return nil, errors.New("trip has no permit assigned")
	}
	log.Printf("[StartTrip] PermitID value: %s", *scheduledTrip.PermitID)

	// Validate that a driver is assigned (required for trip to start)
	log.Printf("[StartTrip] Validating driver assignment...")
	if scheduledTrip.AssignedDriverID == nil {
		log.Printf("[StartTrip] ERROR: No driver assigned - this is the validation that should prevent crash")
		return nil, errors.New("trip cannot start without an assigned driver")
	}
	log.Printf("[StartTrip] Driver validation passed: %s", *scheduledTrip.AssignedDriverID)

	// Get bus from permit
	log.Printf("[StartTrip] Getting permit info...")
	permit, err := s.permitRepo.GetByID(*scheduledTrip.PermitID)
	if err != nil {
		log.Printf("[StartTrip] ERROR: Failed to get permit: %v", err)
		return nil, errors.New("failed to get permit information")
	}
	log.Printf("[StartTrip] Got permit: BusRegNumber=%s", permit.BusRegistrationNumber)

	// Get bus by registration number from permit
	log.Printf("[StartTrip] Getting bus info...")
	bus, err := s.busRepo.GetByLicensePlate(permit.BusRegistrationNumber)
	if err != nil {
		log.Printf("[StartTrip] ERROR: Failed to get bus: %v", err)
		return nil, errors.New("failed to get bus information")
	}
	if bus == nil {
		log.Printf("[StartTrip] ERROR: Bus not found with license plate: %s", permit.BusRegistrationNumber)
		return nil, errors.New("bus not found with license plate: " + permit.BusRegistrationNumber)
	}
	log.Printf("[StartTrip] Got bus: ID=%s", bus.ID)

	// 6. Create the active trip record
	log.Printf("[StartTrip] Creating active trip record...")
	now := time.Now()
	activeTrip := &models.ActiveTrip{
		ScheduledTripID:       input.ScheduledTripID,
		BusID:                 bus.ID,
		PermitID:              *scheduledTrip.PermitID,
		DriverID:              *scheduledTrip.AssignedDriverID, // Safe: validated above
		ConductorID:           scheduledTrip.AssignedConductorID,
		CurrentLatitude:       &input.InitialLatitude,
		CurrentLongitude:      &input.InitialLongitude,
		LastLocationUpdate:    &now,
		Status:                models.ActiveTripStatusInTransit,
		ActualDepartureTime:   &now,
		CurrentPassengerCount: 0,
	}
	log.Printf("[StartTrip] ActiveTrip struct created: DriverID=%s, BusID=%s", activeTrip.DriverID, activeTrip.BusID)

	err = s.activeTripRepo.Create(activeTrip)
	if err != nil {
		log.Printf("[StartTrip] ERROR: Failed to create active trip: %v", err)
		return nil, errors.New("failed to create active trip: " + err.Error())
	}
	log.Printf("[StartTrip] Active trip created successfully: ID=%s", activeTrip.ID)

	// 7. Update scheduled trip status to in_progress
	log.Printf("[StartTrip] Updating scheduled trip status to in_progress...")
	err = s.scheduledTripRepo.UpdateStatus(input.ScheduledTripID, "in_progress")
	if err != nil {
		log.Printf("[StartTrip] WARNING: Failed to update scheduled trip status: %v", err)
		// Log but don't fail - active trip was created successfully
	}

	log.Printf("[StartTrip] === START TRIP COMPLETED SUCCESSFULLY ===")
	return &StartTripResult{
		ActiveTrip:      activeTrip,
		Message:         "Trip started successfully",
		ScheduledTripID: input.ScheduledTripID,
	}, nil
}

// UpdateLocationInput contains location update data
type UpdateLocationInput struct {
	ActiveTripID string   `json:"active_trip_id"`
	StaffID      string   `json:"staff_id"`
	Latitude     float64  `json:"latitude"`
	Longitude    float64  `json:"longitude"`
	SpeedKmh     *float64 `json:"speed_kmh,omitempty"`
	Heading      *float64 `json:"heading,omitempty"`
}

// UpdateLocation updates the current location of an active trip
func (s *activeTripService) UpdateLocation(input *UpdateLocationInput) error {
	// 1. Get the active trip
	activeTrip, err := s.activeTripRepo.GetByID(input.ActiveTripID)
	if err != nil {
		return errors.New("active trip not found")
	}

	// 2. Verify the trip is still active
	if !activeTrip.IsActive() {
		return errors.New("trip is no longer active")
	}

	// 3. Verify the staff is assigned to this trip
	if activeTrip.DriverID != input.StaffID && (activeTrip.ConductorID == nil || *activeTrip.ConductorID != input.StaffID) {
		return errors.New("you are not assigned to this trip")
	}

	// 4. Update location
	err = s.activeTripRepo.UpdateLocation(input.ActiveTripID, input.Latitude, input.Longitude, input.SpeedKmh, input.Heading)
	if err != nil {
		return errors.New("failed to update location: " + err.Error())
	}

	return nil
}

// EndTripInput contains data needed to end a trip
type EndTripInput struct {
	ActiveTripID   string  `json:"active_trip_id"`
	StaffID        string  `json:"staff_id"`
	FinalLatitude  float64 `json:"final_latitude"`
	FinalLongitude float64 `json:"final_longitude"`
}

// EndTripResult contains the result of ending a trip
type EndTripResult struct {
	ActiveTrip *models.ActiveTrip `json:"active_trip"`
	Message    string             `json:"message"`
	Duration   string             `json:"duration"`
}

// EndTrip completes an active trip
func (s *activeTripService) EndTrip(input *EndTripInput) (*EndTripResult, error) {
	// 1. Get the active trip
	activeTrip, err := s.activeTripRepo.GetByID(input.ActiveTripID)
	if err != nil {
		return nil, errors.New("active trip not found")
	}

	// 2. Verify the trip is still active
	if !activeTrip.IsActive() {
		return nil, errors.New("trip is already completed or cancelled")
	}

	// 3. Verify the staff is assigned to this trip
	if activeTrip.DriverID != input.StaffID && (activeTrip.ConductorID == nil || *activeTrip.ConductorID != input.StaffID) {
		return nil, errors.New("you are not assigned to this trip")
	}

	// 4. Update final location
	activeTrip.CurrentLatitude = &input.FinalLatitude
	activeTrip.CurrentLongitude = &input.FinalLongitude

	// 5. Complete the trip
	activeTrip.CompleteTrip()

	// 6. Save the updates
	err = s.activeTripRepo.Update(activeTrip)
	if err != nil {
		return nil, errors.New("failed to complete trip: " + err.Error())
	}

	// 7. Update scheduled trip status to completed
	err = s.scheduledTripRepo.UpdateStatus(activeTrip.ScheduledTripID, "completed")
	if err != nil {
		// Log but don't fail
		// TODO: Add proper logging
	}

	return &EndTripResult{
		ActiveTrip: activeTrip,
		Message:    "Trip completed successfully",
		Duration:   activeTrip.GetTripDuration().String(),
	}, nil
}

// GetActiveTrip retrieves an active trip by ID
func (s *activeTripService) GetActiveTrip(activeTripID string) (*models.ActiveTrip, error) {
	return s.activeTripRepo.GetByID(activeTripID)
}

// GetActiveTripByScheduledTrip retrieves an active trip by scheduled trip ID
func (s *activeTripService) GetActiveTripByScheduledTrip(scheduledTripID string) (*models.ActiveTrip, error) {
	return s.activeTripRepo.GetByScheduledTripID(scheduledTripID)
}

// GetMyActiveTrip gets the current active trip for a staff member
func (s *activeTripService) GetMyActiveTrip(staffID string) (*models.ActiveTrip, error) {
	// Get all active trips and find one for this staff
	activeTrips, err := s.activeTripRepo.GetAllActiveTrips()
	if err != nil {
		return nil, err
	}

	for _, trip := range activeTrips {
		if trip.DriverID == staffID || (trip.ConductorID != nil && *trip.ConductorID == staffID) {
			return &trip, nil
		}
	}

	return nil, errors.New("no active trip found for this staff member")
}

// UpdatePassengerCount updates the current passenger count
func (s *activeTripService) UpdatePassengerCount(activeTripID string, staffID string, count int) error {
	// 1. Get the active trip
	activeTrip, err := s.activeTripRepo.GetByID(activeTripID)
	if err != nil {
		return errors.New("active trip not found")
	}

	// 2. Verify the trip is still active
	if !activeTrip.IsActive() {
		return errors.New("trip is no longer active")
	}

	// 3. Verify the staff is assigned to this trip
	if activeTrip.DriverID != staffID && (activeTrip.ConductorID == nil || *activeTrip.ConductorID != staffID) {
		return errors.New("you are not assigned to this trip")
	}

	// 4. Update passenger count
	activeTrip.CurrentPassengerCount = count
	err = s.activeTripRepo.Update(activeTrip)
	if err != nil {
		return errors.New("failed to update passenger count: " + err.Error())
	}

	return nil
}

// GetPassengersByScheduledTripID gets the list of passengers for a scheduled trip
func (s *activeTripService) GetPassengersByScheduledTripID(scheduledTripID string) ([]models.PassengerInfo, error) {
	// Simply return the passengers for this scheduled trip
	// No need to check if trip is active - staff should be able to see passengers before/during/after trip
	return s.activeTripRepo.GetPassengersByScheduledTripID(scheduledTripID)
}

// UpdatePassengerStatus updates the status of a passenger booking
func (s *activeTripService) UpdatePassengerStatus(bookingID string, status string) error {
	// For now, just update the booking status directly
	// In the future we could add more validation here
	return s.appBookingRepo.UpdateBookingStatusByID(bookingID, status, nil)
}

// BoardPassenger marks a passenger as boarded
func (s *activeTripService) BoardPassenger(bookingID string) error {
	now := time.Now()
	return s.appBookingRepo.UpdateBookingStatusByID(bookingID, "boarded", &now)
}

// VerifyPassengerBooking checks and verifies a passenger booking by reference number
func (s *activeTripService) VerifyPassengerBooking(reference string, scheduledTripID string) (*models.MasterBooking, error) {
	// Verify the scheduled trip is active
	activeTrip, err := s.activeTripRepo.GetByScheduledTripID(scheduledTripID)
	if err != nil {
		return nil, fmt.Errorf("no active trip found: %w", err)
	}

	if !activeTrip.IsActive() {
		return nil, fmt.Errorf("trip is not active")
	}

	// Find and verify booking
	booking, err := s.appBookingRepo.VerifyBookingByReferenceAndTrip(reference, scheduledTripID)
	if err != nil {
		return nil, fmt.Errorf("booking not found or invalid: %w", err)
	}

	return booking, nil
}
