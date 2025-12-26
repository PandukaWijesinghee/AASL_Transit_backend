package services

import (
	"errors"
	"time"

	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// ActiveTripService handles business logic for active trips (real-time trip tracking)
type ActiveTripService struct {
	activeTripRepo    *database.ActiveTripRepository
	scheduledTripRepo *database.ScheduledTripRepository
	staffRepo         *database.BusStaffRepository
	busRepo           *database.BusRepository
	permitRepo        *database.RoutePermitRepository
}

// NewActiveTripService creates a new ActiveTripService
func NewActiveTripService(
	activeTripRepo *database.ActiveTripRepository,
	scheduledTripRepo *database.ScheduledTripRepository,
	staffRepo *database.BusStaffRepository,
	busRepo *database.BusRepository,
	permitRepo *database.RoutePermitRepository,
) *ActiveTripService {
	return &ActiveTripService{
		activeTripRepo:    activeTripRepo,
		scheduledTripRepo: scheduledTripRepo,
		staffRepo:         staffRepo,
		busRepo:           busRepo,
		permitRepo:        permitRepo,
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
func (s *ActiveTripService) StartTrip(input *StartTripInput) (*StartTripResult, error) {
	// 1. Get the scheduled trip
	scheduledTrip, err := s.scheduledTripRepo.GetByID(input.ScheduledTripID)
	if err != nil {
		return nil, errors.New("scheduled trip not found")
	}

	// 2. Validate the scheduled trip can be started
	if scheduledTrip.Status != "scheduled" && scheduledTrip.Status != "confirmed" {
		return nil, errors.New("trip cannot be started - current status: " + string(scheduledTrip.Status))
	}

	// 3. Verify the staff is assigned to this trip
	isDriver := scheduledTrip.AssignedDriverID != nil && *scheduledTrip.AssignedDriverID == input.StaffID
	isConductor := scheduledTrip.AssignedConductorID != nil && *scheduledTrip.AssignedConductorID == input.StaffID
	if !isDriver && !isConductor {
		return nil, errors.New("you are not assigned to this trip")
	}

	// 4. Check if an active trip already exists for this scheduled trip
	existingActiveTrip, err := s.activeTripRepo.GetByScheduledTripID(input.ScheduledTripID)
	if err == nil && existingActiveTrip != nil {
		// Active trip already exists
		if existingActiveTrip.IsActive() {
			return &StartTripResult{
				ActiveTrip:      existingActiveTrip,
				Message:         "Trip already started",
				ScheduledTripID: input.ScheduledTripID,
			}, nil
		}
		// Trip was completed/cancelled, can't restart
		return nil, errors.New("trip has already been completed or cancelled")
	}

	// 5. Get bus and permit info from scheduled trip
	if scheduledTrip.PermitID == nil {
		return nil, errors.New("trip has no permit assigned")
	}
	
	// Validate that a driver is assigned (required for trip to start)
	if scheduledTrip.AssignedDriverID == nil {
		return nil, errors.New("trip cannot start without an assigned driver")
	}

	// Get bus from permit
	permit, err := s.permitRepo.GetByID(*scheduledTrip.PermitID)
	if err != nil {
		return nil, errors.New("failed to get permit information")
	}

	// Get bus by registration number from permit
	bus, err := s.busRepo.GetByLicensePlate(permit.BusRegistrationNumber)
	if err != nil {
		return nil, errors.New("failed to get bus information")
	}

	// 6. Create the active trip record
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

	err = s.activeTripRepo.Create(activeTrip)
	if err != nil {
		return nil, errors.New("failed to create active trip: " + err.Error())
	}

	// 7. Update scheduled trip status to in_progress
	err = s.scheduledTripRepo.UpdateStatus(input.ScheduledTripID, "in_progress")
	if err != nil {
		// Log but don't fail - active trip was created successfully
		// TODO: Add proper logging
	}

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
func (s *ActiveTripService) UpdateLocation(input *UpdateLocationInput) error {
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
func (s *ActiveTripService) EndTrip(input *EndTripInput) (*EndTripResult, error) {
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
func (s *ActiveTripService) GetActiveTrip(activeTripID string) (*models.ActiveTrip, error) {
	return s.activeTripRepo.GetByID(activeTripID)
}

// GetActiveTripByScheduledTrip retrieves an active trip by scheduled trip ID
func (s *ActiveTripService) GetActiveTripByScheduledTrip(scheduledTripID string) (*models.ActiveTrip, error) {
	return s.activeTripRepo.GetByScheduledTripID(scheduledTripID)
}

// GetMyActiveTrip gets the current active trip for a staff member
func (s *ActiveTripService) GetMyActiveTrip(staffID string) (*models.ActiveTrip, error) {
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
func (s *ActiveTripService) UpdatePassengerCount(activeTripID string, staffID string, count int) error {
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
