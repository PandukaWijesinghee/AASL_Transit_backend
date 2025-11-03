package models

import (
	"time"
)

// MasterRoute represents a government-defined bus route
type MasterRoute struct {
	ID                       string   `json:"id" db:"id"`
	RouteNumber              string   `json:"route_number" db:"route_number"`
	RouteName                string   `json:"route_name" db:"route_name"`
	OriginCity               string   `json:"origin_city" db:"origin_city"`
	DestinationCity          string   `json:"destination_city" db:"destination_city"`
	TotalDistanceKm          *float64 `json:"total_distance_km,omitempty" db:"total_distance_km"`
	EstimatedDurationMinutes *int     `json:"estimated_duration_minutes,omitempty" db:"estimated_duration_minutes"`
	EncodedPolyline          *string  `json:"encoded_polyline,omitempty" db:"encoded_polyline"`
	IsActive                 bool     `json:"is_active" db:"is_active"`
	CreatedAt                time.Time `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time `json:"updated_at" db:"updated_at"`
}

// MasterRouteStop represents a predefined stop on a master route
type MasterRouteStop struct {
	ID                      string   `json:"id" db:"id"`
	MasterRouteID           string   `json:"master_route_id" db:"master_route_id"`
	StopName                string   `json:"stop_name" db:"stop_name"`
	StopOrder               int      `json:"stop_order" db:"stop_order"`
	Latitude                *float64 `json:"latitude,omitempty" db:"latitude"`
	Longitude               *float64 `json:"longitude,omitempty" db:"longitude"`
	ArrivalTimeOffsetMinutes *int    `json:"arrival_time_offset_minutes,omitempty" db:"arrival_time_offset_minutes"`
	IsMajorStop             bool     `json:"is_major_stop" db:"is_major_stop"`
	CreatedAt               time.Time `json:"created_at" db:"created_at"`
}

// RouteDisplayName returns a formatted route display name
func (m *MasterRoute) RouteDisplayName() string {
	return m.RouteNumber + ": " + m.OriginCity + " - " + m.DestinationCity
}

// HasPolyline checks if the route has an encoded polyline
func (m *MasterRoute) HasPolyline() bool {
	return m.EncodedPolyline != nil && *m.EncodedPolyline != ""
}
