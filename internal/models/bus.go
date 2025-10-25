package models

import (
	"errors"
	"time"
)

// BusType represents the type/category of bus
type BusType string

const (
	BusTypeNormal      BusType = "normal"
	BusTypeLuxury      BusType = "luxury"
	BusTypeSemiLuxury  BusType = "semi_luxury"
	BusTypeSuperLuxury BusType = "super_luxury"
)

// BusStatus represents the current operational status of a bus
type BusStatus string

const (
	BusStatusActive      BusStatus = "active"
	BusStatusMaintenance BusStatus = "maintenance"
	BusStatusInactive    BusStatus = "inactive"
)

// Bus represents a bus owned by a bus owner
type Bus struct {
	ID                   string     `json:"id" db:"id"`
	BusOwnerID           string     `json:"bus_owner_id" db:"bus_owner_id"`
	PermitID             string     `json:"permit_id" db:"permit_id"`
	BusNumber            string     `json:"bus_number" db:"bus_number"`
	LicensePlate         string     `json:"license_plate" db:"license_plate"`
	BusType              BusType    `json:"bus_type" db:"bus_type"`
	TotalSeats           int        `json:"total_seats" db:"total_seats"`
	ManufacturingYear    *int       `json:"manufacturing_year,omitempty" db:"manufacturing_year"`
	LastMaintenanceDate  *time.Time `json:"last_maintenance_date,omitempty" db:"last_maintenance_date"`
	InsuranceExpiry      *time.Time `json:"insurance_expiry,omitempty" db:"insurance_expiry"`
	Status               BusStatus  `json:"status" db:"status"`

	// Amenities
	HasWifi          bool `json:"has_wifi" db:"has_wifi"`
	HasAC            bool `json:"has_ac" db:"has_ac"`
	HasChargingPorts bool `json:"has_charging_ports" db:"has_charging_ports"`
	HasEntertainment bool `json:"has_entertainment" db:"has_entertainment"`
	HasRefreshments  bool `json:"has_refreshments" db:"has_refreshments"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CreateBusRequest represents the request to create a new bus
type CreateBusRequest struct {
	PermitID            string  `json:"permit_id" binding:"required"`
	BusNumber           string  `json:"bus_number" binding:"required"`
	BusType             string  `json:"bus_type" binding:"required"`
	TotalSeats          int     `json:"total_seats" binding:"required,gt=0"`
	ManufacturingYear   *int    `json:"manufacturing_year,omitempty"`
	LastMaintenanceDate *string `json:"last_maintenance_date,omitempty"` // Format: YYYY-MM-DD
	InsuranceExpiry     *string `json:"insurance_expiry,omitempty"`      // Format: YYYY-MM-DD
	Status              *string `json:"status,omitempty"`

	// Amenities
	HasWifi          bool `json:"has_wifi"`
	HasAC            bool `json:"has_ac"`
	HasChargingPorts bool `json:"has_charging_ports"`
	HasEntertainment bool `json:"has_entertainment"`
	HasRefreshments  bool `json:"has_refreshments"`
}

// UpdateBusRequest represents the request to update bus information
type UpdateBusRequest struct {
	BusNumber           *string `json:"bus_number,omitempty"`
	BusType             *string `json:"bus_type,omitempty"`
	TotalSeats          *int    `json:"total_seats,omitempty"`
	ManufacturingYear   *int    `json:"manufacturing_year,omitempty"`
	LastMaintenanceDate *string `json:"last_maintenance_date,omitempty"` // Format: YYYY-MM-DD
	InsuranceExpiry     *string `json:"insurance_expiry,omitempty"`      // Format: YYYY-MM-DD
	Status              *string `json:"status,omitempty"`

	// Amenities
	HasWifi          *bool `json:"has_wifi,omitempty"`
	HasAC            *bool `json:"has_ac,omitempty"`
	HasChargingPorts *bool `json:"has_charging_ports,omitempty"`
	HasEntertainment *bool `json:"has_entertainment,omitempty"`
	HasRefreshments  *bool `json:"has_refreshments,omitempty"`
}

// Validate validates the CreateBusRequest
func (req *CreateBusRequest) Validate() error {
	// Validate bus type
	busType := BusType(req.BusType)
	if busType != BusTypeNormal && busType != BusTypeLuxury &&
	   busType != BusTypeSemiLuxury && busType != BusTypeSuperLuxury {
		return errors.New("invalid bus_type: must be normal, luxury, semi_luxury, or super_luxury")
	}

	// Validate total seats
	if req.TotalSeats <= 0 {
		return errors.New("total_seats must be greater than 0")
	}

	// Validate manufacturing year if provided
	if req.ManufacturingYear != nil {
		currentYear := time.Now().Year()
		if *req.ManufacturingYear < 1900 || *req.ManufacturingYear > currentYear+1 {
			return errors.New("invalid manufacturing_year")
		}
	}

	// Validate status if provided
	if req.Status != nil {
		status := BusStatus(*req.Status)
		if status != BusStatusActive && status != BusStatusMaintenance && status != BusStatusInactive {
			return errors.New("invalid status: must be active, maintenance, or inactive")
		}
	}

	return nil
}

// Validate validates the UpdateBusRequest
func (req *UpdateBusRequest) Validate() error {
	// Validate bus type if provided
	if req.BusType != nil {
		busType := BusType(*req.BusType)
		if busType != BusTypeNormal && busType != BusTypeLuxury &&
		   busType != BusTypeSemiLuxury && busType != BusTypeSuperLuxury {
			return errors.New("invalid bus_type: must be normal, luxury, semi_luxury, or super_luxury")
		}
	}

	// Validate total seats if provided
	if req.TotalSeats != nil && *req.TotalSeats <= 0 {
		return errors.New("total_seats must be greater than 0")
	}

	// Validate manufacturing year if provided
	if req.ManufacturingYear != nil {
		currentYear := time.Now().Year()
		if *req.ManufacturingYear < 1900 || *req.ManufacturingYear > currentYear+1 {
			return errors.New("invalid manufacturing_year")
		}
	}

	// Validate status if provided
	if req.Status != nil {
		status := BusStatus(*req.Status)
		if status != BusStatusActive && status != BusStatusMaintenance && status != BusStatusInactive {
			return errors.New("invalid status: must be active, maintenance, or inactive")
		}
	}

	return nil
}
