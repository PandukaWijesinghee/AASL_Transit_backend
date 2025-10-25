package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusOwnerRepository handles database operations for bus_owners table
type BusOwnerRepository struct {
	db DB
}

// NewBusOwnerRepository creates a new BusOwnerRepository
func NewBusOwnerRepository(db DB) *BusOwnerRepository {
	return &BusOwnerRepository{db: db}
}

// Create creates a new bus owner record
func (r *BusOwnerRepository) Create(userID, phone string) (*models.BusOwner, error) {
	owner := &models.BusOwner{
		ID:                 uuid.New().String(),
		UserID:             userID,
		VerificationStatus: "pending",
		ProfileCompleted:   false,
	}

	query := `
		INSERT INTO bus_owners (
			id, user_id, verification_status, profile_completed,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		owner.ID,
		owner.UserID,
		owner.VerificationStatus,
		owner.ProfileCompleted,
	).Scan(&owner.CreatedAt, &owner.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create bus owner: %w", err)
	}

	return owner, nil
}

// GetByID retrieves bus owner by ID
func (r *BusOwnerRepository) GetByID(ownerID string) (*models.BusOwner, error) {
	query := `
		SELECT
			id, user_id, company_name, license_number, contact_person,
			address, city, state, country, postal_code, verification_status,
			verification_documents, business_email, business_phone, tax_id,
			bank_account_details, total_buses, profile_completed,
			identity_or_incorporation_no, created_at, updated_at
		FROM bus_owners
		WHERE id = $1
	`

	owner := &models.BusOwner{}
	err := r.db.QueryRow(query, ownerID).Scan(
		&owner.ID, &owner.UserID, &owner.CompanyName, &owner.LicenseNumber,
		&owner.ContactPerson, &owner.Address, &owner.City, &owner.State,
		&owner.Country, &owner.PostalCode, &owner.VerificationStatus,
		&owner.VerificationDocuments, &owner.BusinessEmail, &owner.BusinessPhone,
		&owner.TaxID, &owner.BankAccountDetails, &owner.TotalBuses,
		&owner.ProfileCompleted, &owner.IdentityOrIncorporationNo,
		&owner.CreatedAt, &owner.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("bus owner not found")
		}
		return nil, err
	}

	return owner, nil
}

// GetByUserID retrieves bus owner by user_id
func (r *BusOwnerRepository) GetByUserID(userID string) (*models.BusOwner, error) {
	query := `
		SELECT
			id, user_id, company_name, license_number, contact_person,
			address, city, state, country, postal_code, verification_status,
			verification_documents, business_email, business_phone, tax_id,
			bank_account_details, total_buses, profile_completed,
			identity_or_incorporation_no, created_at, updated_at
		FROM bus_owners
		WHERE user_id = $1
	`

	owner := &models.BusOwner{}
	err := r.db.QueryRow(query, userID).Scan(
		&owner.ID, &owner.UserID, &owner.CompanyName, &owner.LicenseNumber,
		&owner.ContactPerson, &owner.Address, &owner.City, &owner.State,
		&owner.Country, &owner.PostalCode, &owner.VerificationStatus,
		&owner.VerificationDocuments, &owner.BusinessEmail, &owner.BusinessPhone,
		&owner.TaxID, &owner.BankAccountDetails, &owner.TotalBuses,
		&owner.ProfileCompleted, &owner.IdentityOrIncorporationNo,
		&owner.CreatedAt, &owner.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return owner, nil
}

// GetByLicenseNumber retrieves bus owner by license number (can be used as "code")
func (r *BusOwnerRepository) GetByLicenseNumber(licenseNumber string) (*models.BusOwner, error) {
	query := `
		SELECT
			id, user_id, company_name, license_number, contact_person,
			address, city, state, country, postal_code, verification_status,
			verification_documents, business_email, business_phone, tax_id,
			bank_account_details, total_buses, profile_completed,
			identity_or_incorporation_no, created_at, updated_at
		FROM bus_owners
		WHERE license_number = $1 AND verification_status = 'verified'
	`

	owner := &models.BusOwner{}
	err := r.db.QueryRow(query, licenseNumber).Scan(
		&owner.ID, &owner.UserID, &owner.CompanyName, &owner.LicenseNumber,
		&owner.ContactPerson, &owner.Address, &owner.City, &owner.State,
		&owner.Country, &owner.PostalCode, &owner.VerificationStatus,
		&owner.VerificationDocuments, &owner.BusinessEmail, &owner.BusinessPhone,
		&owner.TaxID, &owner.BankAccountDetails, &owner.TotalBuses,
		&owner.ProfileCompleted, &owner.IdentityOrIncorporationNo,
		&owner.CreatedAt, &owner.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("bus owner not found or not verified")
		}
		return nil, err
	}

	return owner, nil
}

// SearchByCompanyName searches bus owners by company name
func (r *BusOwnerRepository) SearchByCompanyName(name string) ([]*models.BusOwner, error) {
	query := `
		SELECT
			id, user_id, company_name, license_number, contact_person,
			address, city, state, country, postal_code, verification_status,
			verification_documents, business_email, business_phone, tax_id,
			bank_account_details, total_buses, profile_completed,
			identity_or_incorporation_no, created_at, updated_at
		FROM bus_owners
		WHERE company_name ILIKE $1 AND verification_status = 'verified'
		ORDER BY company_name
		LIMIT 20
	`

	rows, err := r.db.Query(query, "%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owners := []*models.BusOwner{}
	for rows.Next() {
		owner := &models.BusOwner{}
		err := rows.Scan(
			&owner.ID, &owner.UserID, &owner.CompanyName, &owner.LicenseNumber,
			&owner.ContactPerson, &owner.Address, &owner.City, &owner.State,
			&owner.Country, &owner.PostalCode, &owner.VerificationStatus,
			&owner.VerificationDocuments, &owner.BusinessEmail, &owner.BusinessPhone,
			&owner.TaxID, &owner.BankAccountDetails, &owner.TotalBuses,
			&owner.ProfileCompleted, &owner.IdentityOrIncorporationNo,
			&owner.CreatedAt, &owner.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}

	return owners, nil
}

// GetAllVerified retrieves all verified bus owners
func (r *BusOwnerRepository) GetAllVerified() ([]*models.BusOwner, error) {
	query := `
		SELECT
			id, user_id, company_name, license_number, contact_person,
			address, city, state, country, postal_code, verification_status,
			verification_documents, business_email, business_phone, tax_id,
			bank_account_details, total_buses, profile_completed,
			identity_or_incorporation_no, created_at, updated_at
		FROM bus_owners
		WHERE verification_status = 'verified'
		ORDER BY company_name
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owners := []*models.BusOwner{}
	for rows.Next() {
		owner := &models.BusOwner{}
		err := rows.Scan(
			&owner.ID, &owner.UserID, &owner.CompanyName, &owner.LicenseNumber,
			&owner.ContactPerson, &owner.Address, &owner.City, &owner.State,
			&owner.Country, &owner.PostalCode, &owner.VerificationStatus,
			&owner.VerificationDocuments, &owner.BusinessEmail, &owner.BusinessPhone,
			&owner.TaxID, &owner.BankAccountDetails, &owner.TotalBuses,
			&owner.ProfileCompleted, &owner.IdentityOrIncorporationNo,
			&owner.CreatedAt, &owner.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}

	return owners, nil
}

// UpdateProfile updates bus owner's company profile information
func (r *BusOwnerRepository) UpdateProfile(busOwnerID string, companyName, identityNo string, businessEmail *string) error {
	query := `
		UPDATE bus_owners
		SET company_name = $1,
		    identity_or_incorporation_no = $2,
		    business_email = $3,
		    updated_at = NOW()
		WHERE id = $4
	`

	result, err := r.db.Exec(query, companyName, identityNo, businessEmail, busOwnerID)
	if err != nil {
		return fmt.Errorf("failed to update bus owner profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("bus owner not found")
	}

	return nil
}
