package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusRepository handles database operations for buses
type BusRepository struct {
	db DB
}

// NewBusRepository creates a new BusRepository
func NewBusRepository(db DB) *BusRepository {
	return &BusRepository{db: db}
}

// Create creates a new bus
func (r *BusRepository) Create(bus *models.Bus) error {
	query := `
		INSERT INTO buses (
			id, bus_owner_id, permit_id, bus_number, license_plate,
			bus_type, manufacturing_year, last_maintenance_date,
			insurance_expiry, status, seat_layout_id, has_wifi, has_ac, has_charging_ports,
			has_entertainment, has_refreshments
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		bus.ID, bus.BusOwnerID, bus.PermitID, bus.BusNumber, bus.LicensePlate,
		bus.BusType, bus.ManufacturingYear, bus.LastMaintenanceDate,
		bus.InsuranceExpiry, bus.Status, bus.SeatLayoutID, bus.HasWifi, bus.HasAC, bus.HasChargingPorts,
		bus.HasEntertainment, bus.HasRefreshments,
	).Scan(&bus.CreatedAt, &bus.UpdatedAt)

	return err
}

// GetByID retrieves a bus by ID
func (r *BusRepository) GetByID(busID string) (*models.Bus, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_id, bus_number, license_plate,
			bus_type, manufacturing_year, last_maintenance_date,
			insurance_expiry, status, seat_layout_id, has_wifi, has_ac, has_charging_ports,
			has_entertainment, has_refreshments, created_at, updated_at
		FROM buses
		WHERE id = $1
	`

	bus := &models.Bus{}
	var manufacturingYear sql.NullInt64
	var lastMaintenanceDate sql.NullTime
	var insuranceExpiry sql.NullTime
	var seatLayoutID sql.NullString

	err := r.db.QueryRow(query, busID).Scan(
		&bus.ID, &bus.BusOwnerID, &bus.PermitID, &bus.BusNumber, &bus.LicensePlate,
		&bus.BusType, &manufacturingYear, &lastMaintenanceDate,
		&insuranceExpiry, &bus.Status, &seatLayoutID, &bus.HasWifi, &bus.HasAC, &bus.HasChargingPorts,
		&bus.HasEntertainment, &bus.HasRefreshments, &bus.CreatedAt, &bus.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types to pointers
	if manufacturingYear.Valid {
		year := int(manufacturingYear.Int64)
		bus.ManufacturingYear = &year
	}
	if lastMaintenanceDate.Valid {
		bus.LastMaintenanceDate = &lastMaintenanceDate.Time
	}
	if insuranceExpiry.Valid {
		bus.InsuranceExpiry = &insuranceExpiry.Time
	}
	if seatLayoutID.Valid {
		bus.SeatLayoutID = &seatLayoutID.String
	}

	return bus, nil
}

// GetByOwnerID retrieves all buses for a bus owner
func (r *BusRepository) GetByOwnerID(busOwnerID string) ([]models.Bus, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_id, bus_number, license_plate,
			bus_type, manufacturing_year, last_maintenance_date,
			insurance_expiry, status, seat_layout_id, has_wifi, has_ac, has_charging_ports,
			has_entertainment, has_refreshments, created_at, updated_at
		FROM buses
		WHERE bus_owner_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buses := []models.Bus{}
	for rows.Next() {
		var bus models.Bus
		var manufacturingYear sql.NullInt64
		var lastMaintenanceDate sql.NullTime
		var insuranceExpiry sql.NullTime
		var seatLayoutID sql.NullString

		err := rows.Scan(
			&bus.ID, &bus.BusOwnerID, &bus.PermitID, &bus.BusNumber, &bus.LicensePlate,
			&bus.BusType, &manufacturingYear, &lastMaintenanceDate,
			&insuranceExpiry, &bus.Status, &seatLayoutID, &bus.HasWifi, &bus.HasAC, &bus.HasChargingPorts,
			&bus.HasEntertainment, &bus.HasRefreshments, &bus.CreatedAt, &bus.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types to pointers
		if manufacturingYear.Valid {
			year := int(manufacturingYear.Int64)
			bus.ManufacturingYear = &year
		}
		if lastMaintenanceDate.Valid {
			bus.LastMaintenanceDate = &lastMaintenanceDate.Time
		}
		if insuranceExpiry.Valid {
			bus.InsuranceExpiry = &insuranceExpiry.Time
		}
		if seatLayoutID.Valid {
			bus.SeatLayoutID = &seatLayoutID.String
		}

		buses = append(buses, bus)
	}

	return buses, nil
}

// GetByLicensePlate retrieves a bus by license plate
func (r *BusRepository) GetByLicensePlate(licensePlate string) (*models.Bus, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_id, bus_number, license_plate,
			bus_type, manufacturing_year, last_maintenance_date,
			insurance_expiry, status, seat_layout_id, has_wifi, has_ac, has_charging_ports,
			has_entertainment, has_refreshments, created_at, updated_at
		FROM buses
		WHERE license_plate = $1
	`

	bus := &models.Bus{}
	var manufacturingYear sql.NullInt64
	var lastMaintenanceDate sql.NullTime
	var insuranceExpiry sql.NullTime
	var seatLayoutID sql.NullString

	err := r.db.QueryRow(query, licensePlate).Scan(
		&bus.ID, &bus.BusOwnerID, &bus.PermitID, &bus.BusNumber, &bus.LicensePlate,
		&bus.BusType, &manufacturingYear, &lastMaintenanceDate,
		&insuranceExpiry, &bus.Status, &seatLayoutID, &bus.HasWifi, &bus.HasAC, &bus.HasChargingPorts,
		&bus.HasEntertainment, &bus.HasRefreshments, &bus.CreatedAt, &bus.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert sql.Null* types to pointers
	if manufacturingYear.Valid {
		year := int(manufacturingYear.Int64)
		bus.ManufacturingYear = &year
	}
	if lastMaintenanceDate.Valid {
		bus.LastMaintenanceDate = &lastMaintenanceDate.Time
	}
	if insuranceExpiry.Valid {
		bus.InsuranceExpiry = &insuranceExpiry.Time
	}
	if seatLayoutID.Valid {
		bus.SeatLayoutID = &seatLayoutID.String
	}

	return bus, nil
}

// Update updates a bus
func (r *BusRepository) Update(busID string, req *models.UpdateBusRequest) error {
	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.BusNumber != nil {
		updates = append(updates, fmt.Sprintf("bus_number = $%d", argCount))
		args = append(args, *req.BusNumber)
		argCount++
	}

	if req.BusType != nil {
		updates = append(updates, fmt.Sprintf("bus_type = $%d", argCount))
		args = append(args, *req.BusType)
		argCount++
	}

	if req.ManufacturingYear != nil {
		updates = append(updates, fmt.Sprintf("manufacturing_year = $%d", argCount))
		args = append(args, *req.ManufacturingYear)
		argCount++
	}

	if req.LastMaintenanceDate != nil {
		maintenanceDate, err := time.Parse("2006-01-02", *req.LastMaintenanceDate)
		if err != nil {
			return fmt.Errorf("invalid last_maintenance_date format")
		}
		updates = append(updates, fmt.Sprintf("last_maintenance_date = $%d", argCount))
		args = append(args, maintenanceDate)
		argCount++
	}

	if req.InsuranceExpiry != nil {
		insuranceExpiry, err := time.Parse("2006-01-02", *req.InsuranceExpiry)
		if err != nil {
			return fmt.Errorf("invalid insurance_expiry format")
		}
		updates = append(updates, fmt.Sprintf("insurance_expiry = $%d", argCount))
		args = append(args, insuranceExpiry)
		argCount++
	}

	if req.Status != nil {
		updates = append(updates, fmt.Sprintf("status = $%d", argCount))
		args = append(args, *req.Status)
		argCount++
	}

	if req.HasWifi != nil {
		updates = append(updates, fmt.Sprintf("has_wifi = $%d", argCount))
		args = append(args, *req.HasWifi)
		argCount++
	}

	if req.HasAC != nil {
		updates = append(updates, fmt.Sprintf("has_ac = $%d", argCount))
		args = append(args, *req.HasAC)
		argCount++
	}

	if req.HasChargingPorts != nil {
		updates = append(updates, fmt.Sprintf("has_charging_ports = $%d", argCount))
		args = append(args, *req.HasChargingPorts)
		argCount++
	}

	if req.HasEntertainment != nil {
		updates = append(updates, fmt.Sprintf("has_entertainment = $%d", argCount))
		args = append(args, *req.HasEntertainment)
		argCount++
	}

	if req.HasRefreshments != nil {
		updates = append(updates, fmt.Sprintf("has_refreshments = $%d", argCount))
		args = append(args, *req.HasRefreshments)
		argCount++
	}

	if req.SeatLayoutID != nil {
		updates = append(updates, fmt.Sprintf("seat_layout_id = $%d", argCount))
		args = append(args, *req.SeatLayoutID)
		argCount++
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Add updated_at
	updates = append(updates, "updated_at = NOW()")

	// Add bus ID to args
	args = append(args, busID)

	query := fmt.Sprintf(`
		UPDATE buses
		SET %s
		WHERE id = $%d
	`, strings.Join(updates, ", "), argCount)

	_, err := r.db.Exec(query, args...)
	return err
}

// Delete deletes a bus
func (r *BusRepository) Delete(busID string, busOwnerID string) error {
	query := `DELETE FROM buses WHERE id = $1 AND bus_owner_id = $2`
	result, err := r.db.Exec(query, busID, busOwnerID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetByPermitID retrieves a bus by permit ID (one permit = one bus)
func (r *BusRepository) GetByPermitID(permitID string) (*models.Bus, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_id, bus_number, license_plate,
			bus_type, manufacturing_year, last_maintenance_date,
			insurance_expiry, status, seat_layout_id, has_wifi, has_ac, has_charging_ports,
			has_entertainment, has_refreshments, created_at, updated_at
		FROM buses
		WHERE permit_id = $1
	`

	bus := &models.Bus{}
	var manufacturingYear sql.NullInt64
	var lastMaintenanceDate sql.NullTime
	var insuranceExpiry sql.NullTime
	var seatLayoutID sql.NullString

	err := r.db.QueryRow(query, permitID).Scan(
		&bus.ID, &bus.BusOwnerID, &bus.PermitID, &bus.BusNumber, &bus.LicensePlate,
		&bus.BusType, &manufacturingYear, &lastMaintenanceDate,
		&insuranceExpiry, &bus.Status, &seatLayoutID, &bus.HasWifi, &bus.HasAC, &bus.HasChargingPorts,
		&bus.HasEntertainment, &bus.HasRefreshments, &bus.CreatedAt, &bus.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert sql.Null* types to pointers
	if manufacturingYear.Valid {
		year := int(manufacturingYear.Int64)
		bus.ManufacturingYear = &year
	}
	if lastMaintenanceDate.Valid {
		bus.LastMaintenanceDate = &lastMaintenanceDate.Time
	}
	if insuranceExpiry.Valid {
		bus.InsuranceExpiry = &insuranceExpiry.Time
	}
	if seatLayoutID.Valid {
		bus.SeatLayoutID = &seatLayoutID.String
	}

	return bus, nil
}

// GetByStatus retrieves all buses with a specific status for a bus owner
func (r *BusRepository) GetByStatus(busOwnerID string, status string) ([]models.Bus, error) {
	query := `
		SELECT
			id, bus_owner_id, permit_id, bus_number, license_plate,
			bus_type, manufacturing_year, last_maintenance_date,
			insurance_expiry, status, seat_layout_id, has_wifi, has_ac, has_charging_ports,
			has_entertainment, has_refreshments, created_at, updated_at
		FROM buses
		WHERE bus_owner_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, busOwnerID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buses := []models.Bus{}
	for rows.Next() {
		var bus models.Bus
		var manufacturingYear sql.NullInt64
		var lastMaintenanceDate sql.NullTime
		var insuranceExpiry sql.NullTime
		var seatLayoutID sql.NullString

		err := rows.Scan(
			&bus.ID, &bus.BusOwnerID, &bus.PermitID, &bus.BusNumber, &bus.LicensePlate,
			&bus.BusType, &manufacturingYear, &lastMaintenanceDate,
			&insuranceExpiry, &bus.Status, &seatLayoutID, &bus.HasWifi, &bus.HasAC, &bus.HasChargingPorts,
			&bus.HasEntertainment, &bus.HasRefreshments, &bus.CreatedAt, &bus.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types to pointers
		if manufacturingYear.Valid {
			year := int(manufacturingYear.Int64)
			bus.ManufacturingYear = &year
		}
		if lastMaintenanceDate.Valid {
			bus.LastMaintenanceDate = &lastMaintenanceDate.Time
		}
		if insuranceExpiry.Valid {
			bus.InsuranceExpiry = &insuranceExpiry.Time
		}
		if seatLayoutID.Valid {
			bus.SeatLayoutID = &seatLayoutID.String
		}

		buses = append(buses, bus)
	}

	return buses, nil
}
