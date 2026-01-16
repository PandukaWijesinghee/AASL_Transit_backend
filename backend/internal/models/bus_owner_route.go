package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// BusOwnerRoute represents a custom route configuration created by bus owner
type BusOwnerRoute struct {
	ID               string         `json:"id" db:"id"`
	BusOwnerID       string         `json:"bus_owner_id" db:"bus_owner_id"`
	MasterRouteID    string         `json:"master_route_id" db:"master_route_id"`
	CustomRouteName  string         `json:"custom_route_name" db:"custom_route_name"`
	Direction        string         `json:"direction" db:"direction"` // 'UP' or 'DOWN'
	SelectedStopIDs  pq.StringArray `json:"selected_stop_ids" db:"selected_stop_ids"`
	CreatedAt        time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at" db:"updated_at"`
}

// CreateBusOwnerRouteRequest represents the request to create a custom route
type CreateBusOwnerRouteRequest struct {
	MasterRouteID   string   `json:"master_route_id" binding:"required"`
	CustomRouteName string   `json:"custom_route_name" binding:"required"`
	Direction       string   `json:"direction" binding:"required,oneof=UP DOWN"`
	SelectedStopIDs []string `json:"selected_stop_ids" binding:"required,min=2"`
}

// UpdateBusOwnerRouteRequest represents the request to update a custom route
type UpdateBusOwnerRouteRequest struct {
	CustomRouteName string   `json:"custom_route_name,omitempty"`
	SelectedStopIDs []string `json:"selected_stop_ids,omitempty" binding:"omitempty,min=2"`
}

// Validate validates the CreateBusOwnerRouteRequest
func (r *CreateBusOwnerRouteRequest) Validate() error {
	// Validate UUIDs
	if _, err := uuid.Parse(r.MasterRouteID); err != nil {
		return err
	}

	for _, stopID := range r.SelectedStopIDs {
		if _, err := uuid.Parse(stopID); err != nil {
			return err
		}
	}

	return nil
}
