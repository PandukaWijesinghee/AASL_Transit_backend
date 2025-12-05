package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusStaffRepository handles database operations for bus_staff and bus_staff_employment tables
type BusStaffRepository struct {
	db DB
}

// NewBusStaffRepository creates a new BusStaffRepository
func NewBusStaffRepository(db DB) *BusStaffRepository {
	return &BusStaffRepository{db: db}
}

// GetByUserID retrieves staff record by user_id
func (r *BusStaffRepository) GetByUserID(userID string) (*models.BusStaff, error) {
	query := `
		SELECT 
			id, user_id, first_name, last_name, staff_type, license_number, 
			license_expiry_date, experience_years,
			emergency_contact, emergency_contact_name, 
			profile_completed, is_verified, verification_status,
			verification_notes, verified_at, verified_by, created_at, updated_at
		FROM bus_staff
		WHERE user_id = $1
	`

	staff := &models.BusStaff{}
	err := r.db.QueryRow(query, userID).Scan(
		&staff.ID, &staff.UserID, &staff.FirstName, &staff.LastName, &staff.StaffType,
		&staff.LicenseNumber, &staff.LicenseExpiryDate,
		&staff.ExperienceYears, &staff.EmergencyContact, &staff.EmergencyContactName,
		&staff.ProfileCompleted, &staff.IsVerified, &staff.VerificationStatus,
		&staff.VerificationNotes, &staff.VerifiedAt,
		&staff.VerifiedBy, &staff.CreatedAt, &staff.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("staff not found")
		}
		return nil, err
	}

	return staff, nil
}

// GetByID retrieves staff record by staff ID
func (r *BusStaffRepository) GetByID(staffID string) (*models.BusStaff, error) {
	query := `
		SELECT 
			id, user_id, first_name, last_name, staff_type, license_number, 
			license_expiry_date, experience_years,
			emergency_contact, emergency_contact_name, 
			profile_completed, is_verified, verification_status,
			verification_notes, verified_at, verified_by, created_at, updated_at
		FROM bus_staff
		WHERE id = $1
	`

	staff := &models.BusStaff{}
	err := r.db.QueryRow(query, staffID).Scan(
		&staff.ID, &staff.UserID, &staff.FirstName, &staff.LastName, &staff.StaffType,
		&staff.LicenseNumber, &staff.LicenseExpiryDate,
		&staff.ExperienceYears, &staff.EmergencyContact, &staff.EmergencyContactName,
		&staff.ProfileCompleted, &staff.IsVerified, &staff.VerificationStatus,
		&staff.VerificationNotes, &staff.VerifiedAt,
		&staff.VerifiedBy, &staff.CreatedAt, &staff.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("staff not found")
		}
		return nil, err
	}

	return staff, nil
}

// Create creates a new bus_staff record
func (r *BusStaffRepository) Create(staff *models.BusStaff) error {
	query := `
		INSERT INTO bus_staff (
			user_id, first_name, last_name, staff_type, license_number, 
			license_expiry_date, experience_years, emergency_contact, 
			emergency_contact_name, profile_completed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at, is_verified, verification_status
	`

	err := r.db.QueryRow(
		query,
		staff.UserID,
		staff.FirstName,
		staff.LastName,
		staff.StaffType,
		staff.LicenseNumber,
		staff.LicenseExpiryDate,
		staff.ExperienceYears,
		staff.EmergencyContact,
		staff.EmergencyContactName,
		staff.ProfileCompleted,
	).Scan(
		&staff.ID,
		&staff.CreatedAt,
		&staff.UpdatedAt,
		&staff.IsVerified,
		&staff.VerificationStatus,
	)

	return err
}

// Update updates an existing bus_staff record
func (r *BusStaffRepository) Update(staff *models.BusStaff) error {
	query := `
		UPDATE bus_staff
		SET 
			first_name = $2,
			last_name = $3,
			staff_type = $4,
			license_number = $5,
			license_expiry_date = $6,
			experience_years = $7,
			emergency_contact = $8,
			emergency_contact_name = $9,
			profile_completed = $10,
			is_verified = $11,
			verification_status = $12,
			verification_notes = $13,
			verified_at = $14,
			verified_by = $15,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		staff.ID,
		staff.FirstName,
		staff.LastName,
		staff.StaffType,
		staff.LicenseNumber,
		staff.LicenseExpiryDate,
		staff.ExperienceYears,
		staff.EmergencyContact,
		staff.EmergencyContactName,
		staff.ProfileCompleted,
		staff.IsVerified,
		staff.VerificationStatus,
		staff.VerificationNotes,
		staff.VerifiedAt,
		staff.VerifiedBy,
	).Scan(&staff.UpdatedAt)

	return err
}

// UpdateFields updates specific fields of a staff record
func (r *BusStaffRepository) UpdateFields(userID string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Build dynamic query
	query := "UPDATE bus_staff SET "
	args := []interface{}{}
	argPos := 1

	for field, value := range fields {
		if argPos > 1 {
			query += ", "
		}
		query += fmt.Sprintf("%s = $%d", field, argPos)
		args = append(args, value)
		argPos++
	}

	// Add updated_at
	query += fmt.Sprintf(", updated_at = $%d", argPos)
	args = append(args, time.Now())
	argPos++

	// Add WHERE clause
	query += fmt.Sprintf(" WHERE user_id = $%d", argPos)
	args = append(args, userID)

	_, err := r.db.Exec(query, args...)
	return err
}

// GetByPhoneNumber retrieves staff record by phone number (via users table)
func (r *BusStaffRepository) GetByPhoneNumber(phoneNumber string) (*models.BusStaff, error) {
	query := `
		SELECT 
			bs.id, bs.user_id, bs.first_name, bs.last_name, bs.staff_type, bs.license_number, 
			bs.license_expiry_date, bs.experience_years,
			bs.emergency_contact, bs.emergency_contact_name, 
			bs.profile_completed, bs.is_verified, bs.verification_status,
			bs.verification_notes, bs.verified_at, bs.verified_by, bs.created_at, bs.updated_at
		FROM bus_staff bs
		INNER JOIN users u ON bs.user_id = u.id
		WHERE u.phone = $1
	`

	staff := &models.BusStaff{}
	err := r.db.QueryRow(query, phoneNumber).Scan(
		&staff.ID, &staff.UserID, &staff.FirstName, &staff.LastName, &staff.StaffType,
		&staff.LicenseNumber, &staff.LicenseExpiryDate,
		&staff.ExperienceYears, &staff.EmergencyContact, &staff.EmergencyContactName,
		&staff.ProfileCompleted, &staff.IsVerified, &staff.VerificationStatus,
		&staff.VerificationNotes, &staff.VerifiedAt,
		&staff.VerifiedBy, &staff.CreatedAt, &staff.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil, nil to indicate not found (not an error)
		}
		return nil, err
	}

	return staff, nil
}

// ========== Employment Methods ==========

// GetCurrentEmployment gets the current (is_current=true) employment for a staff member
func (r *BusStaffRepository) GetCurrentEmployment(staffID string) (*models.BusStaffEmployment, error) {
	query := `
		SELECT 
			id, staff_id, bus_owner_id, employment_status, hire_date,
			termination_date, termination_reason, salary_amount,
			performance_rating, total_trips_completed, is_current,
			notes, created_at, updated_at
		FROM bus_staff_employment
		WHERE staff_id = $1 AND is_current = true
	`

	emp := &models.BusStaffEmployment{}
	err := r.db.QueryRow(query, staffID).Scan(
		&emp.ID, &emp.StaffID, &emp.BusOwnerID, &emp.EmploymentStatus, &emp.HireDate,
		&emp.TerminationDate, &emp.TerminationReason, &emp.SalaryAmount,
		&emp.PerformanceRating, &emp.TotalTripsCompleted, &emp.IsCurrent,
		&emp.Notes, &emp.CreatedAt, &emp.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No current employment
		}
		return nil, err
	}

	return emp, nil
}

// GetEmploymentHistory gets all employment records for a staff member
func (r *BusStaffRepository) GetEmploymentHistory(staffID string) ([]*models.BusStaffEmployment, error) {
	query := `
		SELECT 
			id, staff_id, bus_owner_id, employment_status, hire_date,
			termination_date, termination_reason, salary_amount,
			performance_rating, total_trips_completed, is_current,
			notes, created_at, updated_at
		FROM bus_staff_employment
		WHERE staff_id = $1
		ORDER BY hire_date DESC
	`

	rows, err := r.db.Query(query, staffID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*models.BusStaffEmployment
	for rows.Next() {
		emp := &models.BusStaffEmployment{}
		err := rows.Scan(
			&emp.ID, &emp.StaffID, &emp.BusOwnerID, &emp.EmploymentStatus, &emp.HireDate,
			&emp.TerminationDate, &emp.TerminationReason, &emp.SalaryAmount,
			&emp.PerformanceRating, &emp.TotalTripsCompleted, &emp.IsCurrent,
			&emp.Notes, &emp.CreatedAt, &emp.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, emp)
	}

	return history, nil
}

// CreateEmployment creates a new employment record (links staff to bus owner)
func (r *BusStaffRepository) CreateEmployment(employment *models.BusStaffEmployment) error {
	query := `
		INSERT INTO bus_staff_employment (
			staff_id, bus_owner_id, employment_status, hire_date,
			salary_amount, is_current, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, performance_rating, total_trips_completed, created_at, updated_at
	`

	return r.db.QueryRow(
		query,
		employment.StaffID,
		employment.BusOwnerID,
		employment.EmploymentStatus,
		employment.HireDate,
		employment.SalaryAmount,
		employment.IsCurrent,
		employment.Notes,
	).Scan(
		&employment.ID,
		&employment.PerformanceRating,
		&employment.TotalTripsCompleted,
		&employment.CreatedAt,
		&employment.UpdatedAt,
	)
}

// EndEmployment ends the current employment (sets is_current=false, adds termination info)
func (r *BusStaffRepository) EndEmployment(staffID string, status models.EmploymentStatus, reason string) error {
	query := `
		UPDATE bus_staff_employment
		SET 
			is_current = false,
			employment_status = $2,
			termination_date = CURRENT_DATE,
			termination_reason = $3,
			updated_at = NOW()
		WHERE staff_id = $1 AND is_current = true
	`

	result, err := r.db.Exec(query, staffID, status, reason)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no active employment found for this staff member")
	}

	return nil
}

// GetAllByBusOwner retrieves all current staff for a bus owner (via employment table)
func (r *BusStaffRepository) GetAllByBusOwner(busOwnerID string) ([]*models.StaffWithEmployment, error) {
	query := `
		SELECT 
			bs.id, bs.user_id, bs.first_name, bs.last_name, bs.staff_type, bs.license_number, 
			bs.license_expiry_date, bs.experience_years,
			bs.emergency_contact, bs.emergency_contact_name, 
			bs.profile_completed, bs.is_verified, bs.verification_status,
			bs.verification_notes, bs.verified_at, bs.verified_by, bs.created_at, bs.updated_at,
			bse.id, bse.staff_id, bse.bus_owner_id, bse.employment_status, bse.hire_date,
			bse.termination_date, bse.termination_reason, bse.salary_amount,
			bse.performance_rating, bse.total_trips_completed, bse.is_current,
			bse.notes, bse.created_at, bse.updated_at
		FROM bus_staff bs
		INNER JOIN bus_staff_employment bse ON bs.id = bse.staff_id
		WHERE bse.bus_owner_id = $1 AND bse.is_current = true
		ORDER BY bse.hire_date DESC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var staffList []*models.StaffWithEmployment
	for rows.Next() {
		staff := &models.BusStaff{}
		emp := &models.BusStaffEmployment{}

		err := rows.Scan(
			&staff.ID, &staff.UserID, &staff.FirstName, &staff.LastName, &staff.StaffType,
			&staff.LicenseNumber, &staff.LicenseExpiryDate,
			&staff.ExperienceYears, &staff.EmergencyContact, &staff.EmergencyContactName,
			&staff.ProfileCompleted, &staff.IsVerified, &staff.VerificationStatus,
			&staff.VerificationNotes, &staff.VerifiedAt,
			&staff.VerifiedBy, &staff.CreatedAt, &staff.UpdatedAt,
			&emp.ID, &emp.StaffID, &emp.BusOwnerID, &emp.EmploymentStatus, &emp.HireDate,
			&emp.TerminationDate, &emp.TerminationReason, &emp.SalaryAmount,
			&emp.PerformanceRating, &emp.TotalTripsCompleted, &emp.IsCurrent,
			&emp.Notes, &emp.CreatedAt, &emp.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		staffList = append(staffList, &models.StaffWithEmployment{
			Staff:      staff,
			Employment: emp,
		})
	}

	return staffList, nil
}

// UpdateEmploymentFields updates specific fields of an employment record
func (r *BusStaffRepository) UpdateEmploymentFields(employmentID string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Build dynamic query
	query := "UPDATE bus_staff_employment SET "
	args := []interface{}{}
	argPos := 1

	for field, value := range fields {
		if argPos > 1 {
			query += ", "
		}
		query += fmt.Sprintf("%s = $%d", field, argPos)
		args = append(args, value)
		argPos++
	}

	// Add updated_at
	query += fmt.Sprintf(", updated_at = $%d", argPos)
	args = append(args, time.Now())
	argPos++

	// Add WHERE clause
	query += fmt.Sprintf(" WHERE id = $%d", argPos)
	args = append(args, employmentID)

	_, err := r.db.Exec(query, args...)
	return err
}
