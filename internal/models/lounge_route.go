package models

import (
	"time"

	"github.com/google/uuid"
)

// LoungeRoute represents the many-to-many relationship between lounges and routes
// Each record specifies which route a lounge serves and the two stops where
// the lounge is located between
type LoungeRoute struct {
	ID            uuid.UUID `json:"id" db:"id"`
	LoungeID      uuid.UUID `json:"lounge_id" db:"lounge_id"`
	MasterRouteID uuid.UUID `json:"master_route_id" db:"master_route_id"`
	StopBeforeID  uuid.UUID `json:"stop_before_id" db:"stop_before_id"`
	StopAfterID   uuid.UUID `json:"stop_after_id" db:"stop_after_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// LoungeRouteRequest is used when adding/updating lounge routes
type LoungeRouteRequest struct {
	MasterRouteID string `json:"master_route_id" binding:"required"`
	StopBeforeID  string `json:"stop_before_id" binding:"required"`
	StopAfterID   string `json:"stop_after_id" binding:"required"`
}
