package database

import (
	"database/sql"
	"fmt"

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

// GetByID retrieves bus owner by ID
func (r *BusOwnerRepository) GetByID(ownerID string) (*models.BusOwner, error) {
	query := `
		SELECT 
			id, user_id, company_name, license_number, contact_person,
			address, city, state, country, postal_code, verification_status,
			verification_documents, business_email, business_phone, tax_id,
			bank_account_details, total_buses, created_at, updated_at
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
			bank_account_details, total_buses, created_at, updated_at
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

// GetByLicenseNumber retrieves bus owner by license number (can be used as "code")
func (r *BusOwnerRepository) GetByLicenseNumber(licenseNumber string) (*models.BusOwner, error) {
	query := `
		SELECT 
			id, user_id, company_name, license_number, contact_person,
			address, city, state, country, postal_code, verification_status,
			verification_documents, business_email, business_phone, tax_id,
			bank_account_details, total_buses, created_at, updated_at
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
			bank_account_details, total_buses, created_at, updated_at
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
			bank_account_details, total_buses, created_at, updated_at
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
			&owner.CreatedAt, &owner.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}

	return owners, nil
}
