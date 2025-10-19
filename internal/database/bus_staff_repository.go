package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusStaffRepository handles database operations for bus_staff table
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
			id, user_id, bus_owner_id, staff_type, license_number, 
			license_expiry_date, license_document_url, experience_years,
			emergency_contact, emergency_contact_name, 
			medical_certificate_expiry, medical_certificate_url,
			background_check_status, background_check_document_url,
			employment_status, hire_date, termination_date, salary_amount,
			performance_rating, total_trips_completed, profile_completed,
			verification_notes, verified_at, verified_by, created_at, updated_at
		FROM bus_staff
		WHERE user_id = $1
	`

	staff := &models.BusStaff{}
	err := r.db.QueryRow(query, userID).Scan(
		&staff.ID, &staff.UserID, &staff.BusOwnerID, &staff.StaffType,
		&staff.LicenseNumber, &staff.LicenseExpiryDate, &staff.LicenseDocumentURL,
		&staff.ExperienceYears, &staff.EmergencyContact, &staff.EmergencyContactName,
		&staff.MedicalCertificateExpiry, &staff.MedicalCertificateURL,
		&staff.BackgroundCheckStatus, &staff.BackgroundCheckDocumentURL,
		&staff.EmploymentStatus, &staff.HireDate, &staff.TerminationDate,
		&staff.SalaryAmount, &staff.PerformanceRating, &staff.TotalTripsCompleted,
		&staff.ProfileCompleted, &staff.VerificationNotes, &staff.VerifiedAt,
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
			id, user_id, bus_owner_id, staff_type, license_number, 
			license_expiry_date, license_document_url, experience_years,
			emergency_contact, emergency_contact_name, 
			medical_certificate_expiry, medical_certificate_url,
			background_check_status, background_check_document_url,
			employment_status, hire_date, termination_date, salary_amount,
			performance_rating, total_trips_completed, profile_completed,
			verification_notes, verified_at, verified_by, created_at, updated_at
		FROM bus_staff
		WHERE id = $1
	`

	staff := &models.BusStaff{}
	err := r.db.QueryRow(query, staffID).Scan(
		&staff.ID, &staff.UserID, &staff.BusOwnerID, &staff.StaffType,
		&staff.LicenseNumber, &staff.LicenseExpiryDate, &staff.LicenseDocumentURL,
		&staff.ExperienceYears, &staff.EmergencyContact, &staff.EmergencyContactName,
		&staff.MedicalCertificateExpiry, &staff.MedicalCertificateURL,
		&staff.BackgroundCheckStatus, &staff.BackgroundCheckDocumentURL,
		&staff.EmploymentStatus, &staff.HireDate, &staff.TerminationDate,
		&staff.SalaryAmount, &staff.PerformanceRating, &staff.TotalTripsCompleted,
		&staff.ProfileCompleted, &staff.VerificationNotes, &staff.VerifiedAt,
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
			user_id, bus_owner_id, staff_type, license_number, 
			license_expiry_date, experience_years, emergency_contact, 
			emergency_contact_name, employment_status, profile_completed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at, background_check_status, 
		          performance_rating, total_trips_completed
	`

	err := r.db.QueryRow(
		query,
		staff.UserID,
		staff.BusOwnerID,
		staff.StaffType,
		staff.LicenseNumber,
		staff.LicenseExpiryDate,
		staff.ExperienceYears,
		staff.EmergencyContact,
		staff.EmergencyContactName,
		staff.EmploymentStatus,
		staff.ProfileCompleted,
	).Scan(
		&staff.ID,
		&staff.CreatedAt,
		&staff.UpdatedAt,
		&staff.BackgroundCheckStatus,
		&staff.PerformanceRating,
		&staff.TotalTripsCompleted,
	)

	return err
}

// Update updates an existing bus_staff record
func (r *BusStaffRepository) Update(staff *models.BusStaff) error {
	query := `
		UPDATE bus_staff
		SET 
			bus_owner_id = $2,
			staff_type = $3,
			license_number = $4,
			license_expiry_date = $5,
			license_document_url = $6,
			experience_years = $7,
			emergency_contact = $8,
			emergency_contact_name = $9,
			medical_certificate_expiry = $10,
			medical_certificate_url = $11,
			background_check_status = $12,
			background_check_document_url = $13,
			employment_status = $14,
			hire_date = $15,
			termination_date = $16,
			salary_amount = $17,
			performance_rating = $18,
			total_trips_completed = $19,
			profile_completed = $20,
			verification_notes = $21,
			verified_at = $22,
			verified_by = $23,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		staff.ID,
		staff.BusOwnerID,
		staff.StaffType,
		staff.LicenseNumber,
		staff.LicenseExpiryDate,
		staff.LicenseDocumentURL,
		staff.ExperienceYears,
		staff.EmergencyContact,
		staff.EmergencyContactName,
		staff.MedicalCertificateExpiry,
		staff.MedicalCertificateURL,
		staff.BackgroundCheckStatus,
		staff.BackgroundCheckDocumentURL,
		staff.EmploymentStatus,
		staff.HireDate,
		staff.TerminationDate,
		staff.SalaryAmount,
		staff.PerformanceRating,
		staff.TotalTripsCompleted,
		staff.ProfileCompleted,
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

// Delete soft deletes a staff record (sets employment_status to terminated)
func (r *BusStaffRepository) Delete(staffID string) error {
	query := `
		UPDATE bus_staff
		SET employment_status = 'terminated',
		    termination_date = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(query, staffID)
	return err
}

// GetAllByBusOwner retrieves all staff for a bus owner
func (r *BusStaffRepository) GetAllByBusOwner(busOwnerID string) ([]*models.BusStaff, error) {
	query := `
		SELECT 
			id, user_id, bus_owner_id, staff_type, license_number, 
			license_expiry_date, license_document_url, experience_years,
			emergency_contact, emergency_contact_name, 
			medical_certificate_expiry, medical_certificate_url,
			background_check_status, background_check_document_url,
			employment_status, hire_date, termination_date, salary_amount,
			performance_rating, total_trips_completed, profile_completed,
			verification_notes, verified_at, verified_by, created_at, updated_at
		FROM bus_staff
		WHERE bus_owner_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, busOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	staffList := []*models.BusStaff{}
	for rows.Next() {
		staff := &models.BusStaff{}
		err := rows.Scan(
			&staff.ID, &staff.UserID, &staff.BusOwnerID, &staff.StaffType,
			&staff.LicenseNumber, &staff.LicenseExpiryDate, &staff.LicenseDocumentURL,
			&staff.ExperienceYears, &staff.EmergencyContact, &staff.EmergencyContactName,
			&staff.MedicalCertificateExpiry, &staff.MedicalCertificateURL,
			&staff.BackgroundCheckStatus, &staff.BackgroundCheckDocumentURL,
			&staff.EmploymentStatus, &staff.HireDate, &staff.TerminationDate,
			&staff.SalaryAmount, &staff.PerformanceRating, &staff.TotalTripsCompleted,
			&staff.ProfileCompleted, &staff.VerificationNotes, &staff.VerifiedAt,
			&staff.VerifiedBy, &staff.CreatedAt, &staff.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		staffList = append(staffList, staff)
	}

	return staffList, nil
}
