package models

import (
	"time"

	"github.com/google/uuid"
)

// BusSeatLayoutTemplate represents a reusable bus seat layout template
type BusSeatLayoutTemplate struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	TemplateName string     `json:"template_name" db:"template_name"`
	TotalRows    int        `json:"total_rows" db:"total_rows"`
	TotalSeats   int        `json:"total_seats" db:"total_seats"`
	Description  *string    `json:"description,omitempty" db:"description"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	CreatedBy    uuid.UUID  `json:"created_by" db:"created_by"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	Seats        []BusSeatLayoutSeat `json:"seats,omitempty" db:"-"`
}

// BusSeatLayoutSeat represents an individual seat in a layout template
type BusSeatLayoutSeat struct {
	ID           uuid.UUID `json:"id" db:"id"`
	TemplateID   uuid.UUID `json:"template_id" db:"template_id"`
	RowNumber    int       `json:"row_number" db:"row_number"`
	RowLabel     string    `json:"row_label" db:"row_label"`
	Position     int       `json:"position" db:"position"`
	SeatNumber   string    `json:"seat_number" db:"seat_number"`
	IsWindowSeat bool      `json:"is_window_seat" db:"is_window_seat"`
	IsAisleSeat  bool      `json:"is_aisle_seat" db:"is_aisle_seat"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// CreateBusSeatLayoutTemplateRequest represents the request to create a new layout template
type CreateBusSeatLayoutTemplateRequest struct {
	TemplateName string                       `json:"template_name" binding:"required"`
	TotalRows    int                          `json:"total_rows" binding:"required,min=1,max=20"`
	Description  *string                      `json:"description"`
	SeatMap      [][]bool                     `json:"seat_map" binding:"required"` // 2D array: [row][position] true=seat exists
}

// UpdateBusSeatLayoutTemplateRequest represents the request to update a layout template
type UpdateBusSeatLayoutTemplateRequest struct {
	TemplateName *string  `json:"template_name"`
	Description  *string  `json:"description"`
	IsActive     *bool    `json:"is_active"`
}

// BusSeatLayoutTemplateResponse represents the detailed response with seats
type BusSeatLayoutTemplateResponse struct {
	ID           uuid.UUID           `json:"id"`
	TemplateName string              `json:"template_name"`
	TotalRows    int                 `json:"total_rows"`
	TotalSeats   int                 `json:"total_seats"`
	Description  *string             `json:"description,omitempty"`
	IsActive     bool                `json:"is_active"`
	CreatedBy    uuid.UUID           `json:"created_by"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	Seats        []BusSeatLayoutSeat `json:"seats"`
	LayoutPreview BusLayoutPreview   `json:"layout_preview"`
}

// BusLayoutPreview represents the visual layout of the bus for frontend display
type BusLayoutPreview struct {
	Rows []BusRow `json:"rows"`
}

// BusRow represents a single row in the bus layout
type BusRow struct {
	RowNumber int        `json:"row_number"`
	RowLabel  string     `json:"row_label"`
	LeftSeats []SeatInfo `json:"left_seats"`
	RightSeats []SeatInfo `json:"right_seats"`
}

// SeatInfo represents seat information for display
type SeatInfo struct {
	Position     int    `json:"position"`
	SeatNumber   string `json:"seat_number"`
	IsWindowSeat bool   `json:"is_window_seat"`
	IsAisleSeat  bool   `json:"is_aisle_seat"`
}
