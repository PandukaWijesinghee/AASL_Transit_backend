package services

import (
	"errors"
	"fmt"

	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// TripRouteValidator validates trip route updates
type TripRouteValidator struct {
	tripScheduleRepo *database.TripScheduleRepository
	routeRepo        *database.BusOwnerRouteRepository
	permitRepo       *database.RoutePermitRepository
}

// NewTripRouteValidator creates a new validator
func NewTripRouteValidator(
	tripScheduleRepo *database.TripScheduleRepository,
	routeRepo *database.BusOwnerRouteRepository,
	permitRepo *database.RoutePermitRepository,
) *TripRouteValidator {
	return &TripRouteValidator{
		tripScheduleRepo: tripScheduleRepo,
		routeRepo:        routeRepo,
		permitRepo:       permitRepo,
	}
}

// ValidateRouteUpdate validates if a trip can update its bus_owner_route_id
// Rules:
// 1. New route must have the SAME master_route_id as schedule's route
// 2. New route must have the SAME is_up_direction as schedule's route
// 3. New route must be valid for the permit (if permit is assigned)
// 4. Different stop selections are OK (this is the purpose of the override)
func (v *TripRouteValidator) ValidateRouteUpdate(
	scheduleID string,
	newRouteID string,
	permitID *string,
) error {
	// Get the schedule to find its default route
	schedule, err := v.tripScheduleRepo.GetByID(scheduleID)
	if err != nil {
		return fmt.Errorf("failed to get schedule: %w", err)
	}

	if schedule.BusOwnerRouteID == nil {
		return errors.New("schedule has no bus_owner_route_id - cannot validate route override")
	}

	// Get the schedule's route (the baseline)
	scheduleRoute, err := v.routeRepo.GetByID(*schedule.BusOwnerRouteID)
	if err != nil {
		return fmt.Errorf("failed to get schedule's route: %w", err)
	}

	// Get the new route being proposed
	newRoute, err := v.routeRepo.GetByID(newRouteID)
	if err != nil {
		return fmt.Errorf("failed to get new route: %w", err)
	}

	// RULE 1: Must have same master_route_id
	if scheduleRoute.MasterRouteID != newRoute.MasterRouteID {
		return fmt.Errorf(
			"route override rejected: new route has different master_route_id (schedule: %s, new: %s) - trips must use the same master route",
			scheduleRoute.MasterRouteID,
			newRoute.MasterRouteID,
		)
	}

	// RULE 2: Must have same direction (UP/DOWN)
	if scheduleRoute.Direction != newRoute.Direction {
		return fmt.Errorf(
			"route override rejected: new route has different direction (schedule: %s, new: %s) - trips cannot reverse direction",
			scheduleRoute.Direction,
			newRoute.Direction,
		)
	}

	// RULE 3: If permit is assigned, verify route is valid for permit
	if permitID != nil {
		permit, err := v.permitRepo.GetByID(*permitID)
		if err != nil {
			return fmt.Errorf("failed to get permit: %w", err)
		}

		// Check if permit's master_route_id matches
		if permit.MasterRouteID != newRoute.MasterRouteID {
			return fmt.Errorf(
				"route override rejected: permit is for different master route (permit: %s, new route: %s)",
				permit.MasterRouteID,
				newRoute.MasterRouteID,
			)
		}
	}

	// All validations passed!
	return nil
}

// GetValidRoutesForTrip returns all valid bus_owner_routes that a trip can use
// These are routes with the same master_route_id and direction as the schedule's route
func (v *TripRouteValidator) GetValidRoutesForTrip(
	busOwnerID string,
	scheduleID string,
) ([]models.BusOwnerRoute, error) {
	// Get the schedule to find its default route
	schedule, err := v.tripScheduleRepo.GetByID(scheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	if schedule.BusOwnerRouteID == nil {
		return nil, errors.New("schedule has no bus_owner_route_id")
	}

	// Get the schedule's route (the baseline)
	scheduleRoute, err := v.routeRepo.GetByID(*schedule.BusOwnerRouteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule's route: %w", err)
	}

	// Get all routes for this bus owner
	allRoutes, err := v.routeRepo.GetByBusOwnerID(busOwnerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bus owner routes: %w", err)
	}

	// Filter to only routes with same master_route_id and direction
	validRoutes := []models.BusOwnerRoute{}
	for _, route := range allRoutes {
		if route.MasterRouteID == scheduleRoute.MasterRouteID &&
			route.Direction == scheduleRoute.Direction {
			validRoutes = append(validRoutes, route)
		}
	}

	return validRoutes, nil
}
