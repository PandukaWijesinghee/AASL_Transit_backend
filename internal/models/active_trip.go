package models

import (
	"time"
)

// ActiveTripStatus represents the status of an active trip
type ActiveTripStatus string

const (
	ActiveTripStatusNotStarted ActiveTripStatus = "not_started"
	ActiveTripStatusInTransit  ActiveTripStatus = "in_transit"
	ActiveTripStatusAtStop     ActiveTripStatus = "at_stop"
	ActiveTripStatusCompleted  ActiveTripStatus = "completed"
	ActiveTripStatusCancelled  ActiveTripStatus = "cancelled"
)

// ActiveTrip represents a real-time tracking of a currently running trip
type ActiveTrip struct {
	ID                   string           `json:"id" db:"id"`
	ScheduledTripID      string           `json:"scheduled_trip_id" db:"scheduled_trip_id"`
	BusID                string           `json:"bus_id" db:"bus_id"`
	PermitID             string           `json:"permit_id" db:"permit_id"`
	DriverID             string           `json:"driver_id" db:"driver_id"`
	ConductorID          *string          `json:"conductor_id,omitempty" db:"conductor_id"`
	CurrentLatitude      *float64         `json:"current_latitude,omitempty" db:"current_latitude"`
	CurrentLongitude     *float64         `json:"current_longitude,omitempty" db:"current_longitude"`
	LastLocationUpdate   *time.Time       `json:"last_location_update,omitempty" db:"last_location_update"`
	CurrentSpeedKmh      *float64         `json:"current_speed_kmh,omitempty" db:"current_speed_kmh"`
	Heading              *float64         `json:"heading,omitempty" db:"heading"` // Compass direction 0-360
	CurrentStopID        *string          `json:"current_stop_id,omitempty" db:"current_stop_id"`
	NextStopID           *string          `json:"next_stop_id,omitempty" db:"next_stop_id"`
	StopsCompleted       UUIDArray        `json:"stops_completed,omitempty" db:"stops_completed"`
	ActualDepartureTime  *time.Time       `json:"actual_departure_time,omitempty" db:"actual_departure_time"`
	EstimatedArrivalTime *time.Time       `json:"estimated_arrival_time,omitempty" db:"estimated_arrival_time"`
	ActualArrivalTime    *time.Time       `json:"actual_arrival_time,omitempty" db:"actual_arrival_time"`
	Status               ActiveTripStatus `json:"status" db:"status"`
	CurrentPassengerCount int             `json:"current_passenger_count" db:"current_passenger_count"`
	TrackingDeviceID     *string          `json:"tracking_device_id,omitempty" db:"tracking_device_id"`
	CreatedAt            time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at" db:"updated_at"`
}

// StartTripRequest represents the request to start a trip
type StartTripRequest struct {
	ScheduledTripID  string  `json:"scheduled_trip_id" binding:"required"`
	DriverID         string  `json:"driver_id" binding:"required"`
	BusID            string  `json:"bus_id" binding:"required"`
	ConductorID      *string `json:"conductor_id,omitempty"`
	TrackingDeviceID *string `json:"tracking_device_id,omitempty"`
}

// UpdateLocationRequest represents the request to update trip location
type UpdateLocationRequest struct {
	Latitude    float64  `json:"latitude" binding:"required"`
	Longitude   float64  `json:"longitude" binding:"required"`
	SpeedKmh    *float64 `json:"speed_kmh,omitempty"`
	Heading     *float64 `json:"heading,omitempty"`
	CurrentStopID *string `json:"current_stop_id,omitempty"`
}

// UpdatePassengerCountRequest represents the request to update passenger count
type UpdatePassengerCountRequest struct {
	PassengerCount int `json:"passenger_count" binding:"required,min=0"`
}

// UpdateLocation updates the current location of the active trip
func (a *ActiveTrip) UpdateLocation(lat, lng float64, speedKmh *float64, heading *float64) {
	a.CurrentLatitude = &lat
	a.CurrentLongitude = &lng
	a.CurrentSpeedKmh = speedKmh
	a.Heading = heading
	now := time.Now()
	a.LastLocationUpdate = &now
	a.UpdatedAt = now
}

// StartTrip marks the trip as started
func (a *ActiveTrip) StartTrip() {
	now := time.Now()
	a.ActualDepartureTime = &now
	a.Status = ActiveTripStatusInTransit
	a.UpdatedAt = now
}

// CompleteTrip marks the trip as completed
func (a *ActiveTrip) CompleteTrip() {
	now := time.Now()
	a.ActualArrivalTime = &now
	a.Status = ActiveTripStatusCompleted
	a.UpdatedAt = now
}

// ArriveAtStop marks arrival at a stop
func (a *ActiveTrip) ArriveAtStop(stopID string) {
	a.CurrentStopID = &stopID
	a.Status = ActiveTripStatusAtStop

	// Add to completed stops if not already there
	if a.StopsCompleted == nil {
		a.StopsCompleted = UUIDArray{stopID}
	} else {
		// Check if not already in completed
		found := false
		for _, completedStopID := range a.StopsCompleted {
			if completedStopID == stopID {
				found = true
				break
			}
		}
		if !found {
			a.StopsCompleted = append(a.StopsCompleted, stopID)
		}
	}

	a.UpdatedAt = time.Now()
}

// DepartFromStop marks departure from a stop
func (a *ActiveTrip) DepartFromStop(nextStopID *string) {
	a.NextStopID = nextStopID
	a.Status = ActiveTripStatusInTransit
	a.UpdatedAt = time.Now()
}

// IsActive checks if the trip is currently active (not completed or cancelled)
func (a *ActiveTrip) IsActive() bool {
	return a.Status != ActiveTripStatusCompleted && a.Status != ActiveTripStatusCancelled
}

// GetTripDuration returns the duration of the trip so far
func (a *ActiveTrip) GetTripDuration() time.Duration {
	if a.ActualDepartureTime == nil {
		return 0
	}

	endTime := time.Now()
	if a.ActualArrivalTime != nil {
		endTime = *a.ActualArrivalTime
	}

	return endTime.Sub(*a.ActualDepartureTime)
}

// HasLocation checks if the trip has current location data
func (a *ActiveTrip) HasLocation() bool {
	return a.CurrentLatitude != nil && a.CurrentLongitude != nil
}

// GetLocationAge returns how old the current location data is
func (a *ActiveTrip) GetLocationAge() time.Duration {
	if a.LastLocationUpdate == nil {
		return 0
	}
	return time.Since(*a.LastLocationUpdate)
}

// IsLocationStale checks if location is older than the specified duration
func (a *ActiveTrip) IsLocationStale(staleDuration time.Duration) bool {
	return a.GetLocationAge() > staleDuration
}
