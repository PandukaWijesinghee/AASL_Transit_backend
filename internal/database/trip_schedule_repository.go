package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// TripScheduleRepository handles database operations for trip_schedules table
type TripScheduleRepository struct {
	db DB
}

// NewTripScheduleRepository creates a new TripScheduleRepository
func NewTripScheduleRepository(db DB) *TripScheduleRepository {
	return &TripScheduleRepository{db: db}
}

// CreateTimetable creates a new timetable (trip schedule) using the new timetable system
func (r *TripScheduleRepository) CreateTimetable(schedule *models.TripSchedule) error {
	query := `
		INSERT INTO trip_schedules (
			id, bus_owner_id, bus_owner_route_id, schedule_name,
			recurrence_type, recurrence_days, recurrence_interval, departure_time,
			estimated_duration_minutes, base_fare, is_active, notes
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		RETURNING created_at, updated_at
	`

	// Generate ID if not provided
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	err := r.db.QueryRow(
		query,
		schedule.ID, schedule.BusOwnerID, schedule.BusOwnerRouteID, schedule.ScheduleName,
		schedule.RecurrenceType, schedule.RecurrenceDays, schedule.RecurrenceInterval, schedule.DepartureTime,
		schedule.EstimatedDurationMinutes, schedule.BaseFare, schedule.IsActive, schedule.Notes,
	).Scan(&schedule.CreatedAt, &schedule.UpdatedAt)

	return err
}

// Deprecated: Create creates a new trip schedule (old system, kept for backward compatibility)
func (r *TripScheduleRepository) Create(schedule *models.TripSchedule) error {
	query := `
		INSERT INTO trip_schedules (
			id, bus_owner_id, permit_id, bus_id, schedule_name,
			recurrence_type, recurrence_days, specific_dates, departure_time,
			base_fare, is_bookable, max_bookable_seats, advance_booking_hours,
			default_driver_id, default_conductor_id, selected_stop_ids,
			is_active, valid_from_old, valid_until_old, notes
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		)
		RETURNING created_at, updated_at
	`

	// Generate ID if not provided
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	err := r.db.QueryRow(
		query,
		schedule.ID, schedule.BusOwnerID, schedule.PermitID, schedule.BusID, schedule.ScheduleName,
		schedule.RecurrenceType, schedule.RecurrenceDays, schedule.SpecificDates, schedule.DepartureTime,
		schedule.BaseFare, schedule.IsBookable, schedule.MaxBookableSeats, schedule.AdvanceBookingHours,
		schedule.DefaultDriverID, schedule.DefaultConductorID, schedule.SelectedStopIDs,
		schedule.IsActive, schedule.ValidFrom, schedule.ValidUntil, schedule.Notes,
	).Scan(&schedule.CreatedAt, &schedule.UpdatedAt)

	return err
}

// GetByID retrieves a trip schedule by ID
func (r *TripScheduleRepository) GetByID(scheduleID string) (*models.TripSchedule, error) {
	query := `
		SELECT id, bus_owner_id, bus_owner_route_id, schedule_name,
			   recurrence_type, recurrence_days, recurrence_interval, 
			   departure_time, estimated_duration_minutes,
			   base_fare, is_active, notes,
			   valid_from, valid_until, specific_dates,
			   created_at, updated_at
		FROM trip_schedules
		WHERE id = $1
	`

	schedule := &models.TripSchedule{}
	var customRouteID sql.NullString
	var scheduleName sql.NullString
	var recurrenceInterval sql.NullInt64
	var estimatedDurationMinutes sql.NullInt64
	var validFrom sql.NullTime
	var validUntil sql.NullTime
	var notes sql.NullString

	err := r.db.QueryRow(query, scheduleID).Scan(
		&schedule.ID, &schedule.BusOwnerID, &customRouteID, &scheduleName,
		&schedule.RecurrenceType, &schedule.RecurrenceDays, &recurrenceInterval,
		&schedule.DepartureTime, &estimatedDurationMinutes,
		&schedule.BaseFare, &schedule.IsActive, &notes,
		&validFrom, &validUntil, &schedule.SpecificDates,
		&schedule.CreatedAt, &schedule.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if customRouteID.Valid {
		schedule.BusOwnerRouteID = &customRouteID.String
	}
	if scheduleName.Valid {
		schedule.ScheduleName = &scheduleName.String
	}
	if recurrenceInterval.Valid {
		interval := int(recurrenceInterval.Int64)
		schedule.RecurrenceInterval = &interval
	}
	if estimatedDurationMinutes.Valid {
		duration := int(estimatedDurationMinutes.Int64)
		schedule.EstimatedDurationMinutes = &duration
	}
	if validFrom.Valid {
		schedule.ValidFrom = validFrom.Time
	}
	if validUntil.Valid {
		schedule.ValidUntil = &validUntil.Time
	}
	if notes.Valid {
		schedule.Notes = &notes.String
	}

	return schedule, nil
}

// GetByBusOwnerID retrieves all trip schedules for a bus owner
func (r *TripScheduleRepository) GetByBusOwnerID(busOwnerID string) ([]models.TripSchedule, error) {
	fmt.Printf("🔍 REPO GetByBusOwnerID: Querying for bus_owner_id=%s\n", busOwnerID)

	query := `
		SELECT id, bus_owner_id, bus_owner_route_id, schedule_name,
			   recurrence_type, recurrence_days, recurrence_interval, 
			   departure_time, estimated_duration_minutes,
			   base_fare, is_active, notes,
			   valid_from, valid_until, specific_dates,
			   created_at, updated_at
		FROM trip_schedules
		WHERE bus_owner_id = $1
		ORDER BY departure_time
	`

	fmt.Printf("📝 REPO: Executing query...\n")
	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		fmt.Printf("❌ REPO: Query execution failed: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	fmt.Printf("✅ REPO: Query executed, starting to scan rows...\n")
	schedules, scanErr := r.scanSchedules(rows)
	if scanErr != nil {
		fmt.Printf("❌ REPO: Scan failed: %v\n", scanErr)
		return nil, scanErr
	}

	fmt.Printf("✅ REPO: Successfully scanned %d schedules\n", len(schedules))
	return schedules, nil
}

// GetByPermitID retrieves all trip schedules for a permit
// GetByPermitID retrieves all timetables for a specific permit (DEPRECATED - use bus_owner_route_id)
func (r *TripScheduleRepository) GetByPermitID(permitID string) ([]models.TripSchedule, error) {
	// Note: permit_id doesn't exist in trip_schedules table anymore
	// This function is kept for backward compatibility but returns empty
	return []models.TripSchedule{}, nil
}

// GetByCustomRouteID retrieves all timetables for a specific custom route
func (r *TripScheduleRepository) GetByCustomRouteID(customRouteID string) ([]models.TripSchedule, error) {
	query := `
		SELECT id, bus_owner_id, bus_owner_route_id, schedule_name,
			   recurrence_type, recurrence_days, recurrence_interval, departure_time,
			   estimated_duration_minutes, base_fare, is_active, notes,
			   created_at, updated_at
		FROM trip_schedules
		WHERE bus_owner_route_id = $1
		ORDER BY departure_time
	`

	rows, err := r.db.Query(query, customRouteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTimetables(rows)
}

// GetAllActiveTimetables retrieves all active timetables (for cron job)
func (r *TripScheduleRepository) GetAllActiveTimetables() ([]models.TripSchedule, error) {
	query := `
		SELECT id, bus_owner_id, bus_owner_route_id, schedule_name,
			   recurrence_type, recurrence_days, recurrence_interval, departure_time,
			   estimated_duration_minutes, base_fare, is_active, notes,
			   created_at, updated_at
		FROM trip_schedules
		WHERE is_active = true
		  AND bus_owner_route_id IS NOT NULL
		ORDER BY bus_owner_id, departure_time
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTimetables(rows)
}

// GetActiveSchedulesForDate retrieves all active schedules for a specific date
func (r *TripScheduleRepository) GetActiveSchedulesForDate(date time.Time) ([]models.TripSchedule, error) {
	query := `
		SELECT id, bus_owner_id, bus_owner_route_id, schedule_name,
			   recurrence_type, recurrence_days, recurrence_interval, 
			   departure_time, estimated_arrival_time,
			   base_fare, is_active, notes,
			   valid_from, valid_until, specific_dates,
			   created_at, updated_at
		FROM trip_schedules
		WHERE is_active = true
		  AND valid_from <= $1
		  AND (valid_until IS NULL OR valid_until >= $1)
	`

	rows, err := r.db.Query(query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSchedules(rows)
}

// Update updates a trip schedule
func (r *TripScheduleRepository) Update(schedule *models.TripSchedule) error {
	query := `
		UPDATE trip_schedules
		SET bus_id = $2, schedule_name = $3, recurrence_type = $4,
			recurrence_days = $5, specific_dates = $6, departure_time = $7,
			base_fare = $8, is_bookable = $9, max_bookable_seats = $10,
			advance_booking_hours = $11, default_driver_id = $12,
			default_conductor_id = $13, selected_stop_ids = $14,
			is_active = $15, valid_from = $16, valid_until = $17,
			notes = $18, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		schedule.ID, schedule.BusID, schedule.ScheduleName, schedule.RecurrenceType,
		schedule.RecurrenceDays, schedule.SpecificDates, schedule.DepartureTime,
		schedule.BaseFare, schedule.IsBookable, schedule.MaxBookableSeats,
		schedule.AdvanceBookingHours, schedule.DefaultDriverID,
		schedule.DefaultConductorID, schedule.SelectedStopIDs,
		schedule.IsActive, schedule.ValidFrom, schedule.ValidUntil,
		schedule.Notes,
	).Scan(&schedule.UpdatedAt)

	return err
}

// Delete deletes a trip schedule
func (r *TripScheduleRepository) Delete(scheduleID string) error {
	query := `DELETE FROM trip_schedules WHERE id = $1`
	_, err := r.db.Exec(query, scheduleID)
	return err
}

// Deactivate deactivates a trip schedule
func (r *TripScheduleRepository) Deactivate(scheduleID string) error {
	query := `UPDATE trip_schedules SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, scheduleID)
	return err
}

// scanSchedules scans multiple schedules from rows (NEW SCHEMA - matches GetByBusOwnerID query)
func (r *TripScheduleRepository) scanSchedules(rows *sql.Rows) ([]models.TripSchedule, error) {
	schedules := []models.TripSchedule{}
	rowNum := 0

	for rows.Next() {
		rowNum++
		fmt.Printf("📋 REPO scanSchedules: Processing row #%d\n", rowNum)

		var schedule models.TripSchedule
		var busOwnerRouteID sql.NullString
		var scheduleName sql.NullString
		var recurrenceInterval sql.NullInt64
		var estimatedDurationMinutes sql.NullInt64
		var validFrom sql.NullTime
		var validUntil sql.NullTime
		var notes sql.NullString

		// Scan array columns as TEXT (comma-separated strings)
		var recurrenceDaysStr sql.NullString
		var specificDatesStr sql.NullString

		fmt.Printf("🔍 REPO: About to scan row #%d with columns: id, bus_owner_id, bus_owner_route_id, schedule_name, recurrence_type, recurrence_days...\n", rowNum)

		// Must match the SELECT order from GetByBusOwnerID:
		// id, bus_owner_id, bus_owner_route_id, schedule_name,
		// recurrence_type, recurrence_days, recurrence_interval,
		// departure_time, estimated_duration_minutes,
		// base_fare, is_active, notes,
		// valid_from, valid_until, specific_dates,
		// created_at, updated_at
		err := rows.Scan(
			&schedule.ID, &schedule.BusOwnerID, &busOwnerRouteID, &scheduleName,
			&schedule.RecurrenceType, &recurrenceDaysStr, &recurrenceInterval,
			&schedule.DepartureTime, &estimatedDurationMinutes,
			&schedule.BaseFare, &schedule.IsActive, &notes,
			&validFrom, &validUntil, &specificDatesStr,
			&schedule.CreatedAt, &schedule.UpdatedAt,
		)

		if err != nil {
			fmt.Printf("❌ REPO: Scan FAILED on row #%d: %v\n", rowNum, err)
			fmt.Printf("   Schedule ID (if scanned): %s\n", schedule.ID)
			return nil, err
		}

		// Convert TEXT columns to string fields (empty if NULL)
		if recurrenceDaysStr.Valid {
			schedule.RecurrenceDays = recurrenceDaysStr.String
		} else {
			schedule.RecurrenceDays = ""
		}
		if specificDatesStr.Valid {
			dateStr := specificDatesStr.String
			schedule.SpecificDates = &dateStr
		} else {
			schedule.SpecificDates = nil
		}

		specificDatesDisplay := ""
		if schedule.SpecificDates != nil {
			specificDatesDisplay = *schedule.SpecificDates
		}
		fmt.Printf("✅ REPO: Row #%d scanned successfully - ID=%s, RecurrenceDays=%s, SpecificDates=%s\n",
			rowNum, schedule.ID, schedule.RecurrenceDays, specificDatesDisplay)

		// Convert sql.Null* types
		if busOwnerRouteID.Valid {
			schedule.BusOwnerRouteID = &busOwnerRouteID.String
		}
		if scheduleName.Valid {
			schedule.ScheduleName = &scheduleName.String
		}
		if recurrenceInterval.Valid {
			interval := int(recurrenceInterval.Int64)
			schedule.RecurrenceInterval = &interval
		}
		if estimatedDurationMinutes.Valid {
			duration := int(estimatedDurationMinutes.Int64)
			schedule.EstimatedDurationMinutes = &duration
		}
		if validFrom.Valid {
			schedule.ValidFrom = validFrom.Time
		}
		if validUntil.Valid {
			schedule.ValidUntil = &validUntil.Time
		}
		if notes.Valid {
			schedule.Notes = &notes.String
		}

		schedules = append(schedules, schedule)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return schedules, nil
}

// scanTimetables scans multiple timetables from rows (new system)
func (r *TripScheduleRepository) scanTimetables(rows *sql.Rows) ([]models.TripSchedule, error) {
	timetables := []models.TripSchedule{}

	for rows.Next() {
		var schedule models.TripSchedule
		var customRouteID sql.NullString
		var scheduleName sql.NullString
		var recurrenceInterval sql.NullInt64
		var estimatedDurationMinutes sql.NullInt64
		var notes sql.NullString

		err := rows.Scan(
			&schedule.ID, &schedule.BusOwnerID, &customRouteID, &scheduleName,
			&schedule.RecurrenceType, &schedule.RecurrenceDays, &recurrenceInterval, &schedule.DepartureTime,
			&estimatedDurationMinutes, &schedule.BaseFare, &schedule.IsActive, &notes,
			&schedule.CreatedAt, &schedule.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if customRouteID.Valid {
			schedule.BusOwnerRouteID = &customRouteID.String
		}
		if scheduleName.Valid {
			schedule.ScheduleName = &scheduleName.String
		}
		if recurrenceInterval.Valid {
			interval := int(recurrenceInterval.Int64)
			schedule.RecurrenceInterval = &interval
		}
		if estimatedDurationMinutes.Valid {
			duration := int(estimatedDurationMinutes.Int64)
			schedule.EstimatedDurationMinutes = &duration
		}
		if notes.Valid {
			schedule.Notes = &notes.String
		}

		timetables = append(timetables, schedule)
	}

	return timetables, rows.Err()
}
