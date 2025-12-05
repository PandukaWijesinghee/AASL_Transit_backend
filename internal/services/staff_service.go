package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// StaffService handles business logic for staff operations
type StaffService struct {
	staffRepo *database.BusStaffRepository
	ownerRepo *database.BusOwnerRepository
	userRepo  *database.UserRepository
}

// NewStaffService creates a new StaffService
func NewStaffService(
	staffRepo *database.BusStaffRepository,
	ownerRepo *database.BusOwnerRepository,
	userRepo *database.UserRepository,
) *StaffService {
	return &StaffService{
		staffRepo: staffRepo,
		ownerRepo: ownerRepo,
		userRepo:  userRepo,
	}
}

// RegisterStaff registers a new driver or conductor (profile only, no employment)
func (s *StaffService) RegisterStaff(input *models.StaffRegistrationInput) (*models.BusStaff, error) {
	// Validate staff type
	if input.StaffType != models.StaffTypeDriver && input.StaffType != models.StaffTypeConductor {
		return nil, fmt.Errorf("invalid staff type")
	}

	// Check if user exists
	userUUID, err := uuid.Parse(input.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %v", err)
	}

	_, err = s.userRepo.GetUserByID(userUUID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Check if user already registered as staff
	existingStaff, _ := s.staffRepo.GetByUserID(input.UserID)
	if existingStaff != nil {
		return nil, fmt.Errorf("user already registered as staff")
	}

	// Build staff record (profile only - no employment fields)
	staff := &models.BusStaff{
		UserID:               input.UserID,
		FirstName:            &input.FirstName,
		LastName:             &input.LastName,
		StaffType:            input.StaffType,
		ExperienceYears:      input.ExperienceYears,
		EmergencyContact:     &input.EmergencyContact,
		EmergencyContactName: &input.EmergencyContactName,
		ProfileCompleted:     true, // Mark as completed after initial registration
	}

	// Handle driver-specific fields
	if input.StaffType == models.StaffTypeDriver {
		if input.LicenseNumber != nil && *input.LicenseNumber != "" {
			staff.LicenseNumber = input.LicenseNumber
		}

		if input.LicenseExpiryDate != nil && *input.LicenseExpiryDate != "" {
			expiryDate, err := time.Parse("2006-01-02", *input.LicenseExpiryDate)
			if err == nil {
				staff.LicenseExpiryDate = &expiryDate
			}
		}
	}

	// Create staff record
	err = s.staffRepo.Create(staff)
	if err != nil {
		return nil, fmt.Errorf("failed to create staff record: %v", err)
	}

	// IMPORTANT: Assign role to user based on staff type
	var roleToAdd string
	if staff.StaffType == models.StaffTypeDriver {
		roleToAdd = "driver"
	} else if staff.StaffType == models.StaffTypeConductor {
		roleToAdd = "conductor"
	}

	// Add the role to the user
	err = s.userRepo.AddUserRole(userUUID, roleToAdd)
	if err != nil {
		// Log error but don't fail the registration
		// The staff record is already created
		fmt.Printf("WARNING: Failed to add role %s to user %s: %v\n", roleToAdd, userUUID, err)
	}

	return staff, nil
}

// GetCompleteProfile retrieves complete staff profile with user and current employment info
func (s *StaffService) GetCompleteProfile(userID string) (*models.CompleteStaffProfile, error) {
	// Get user
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %v", err)
	}

	user, err := s.userRepo.GetUserByID(userUUID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Get staff record
	staff, err := s.staffRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("staff record not found: %v", err)
	}

	profile := &models.CompleteStaffProfile{
		User:  user,
		Staff: staff,
	}

	// Get current employment if any
	employment, err := s.staffRepo.GetCurrentEmployment(staff.ID)
	if err == nil && employment != nil {
		profile.Employment = employment

		// Get bus owner info for current employment
		owner, err := s.ownerRepo.GetByID(employment.BusOwnerID)
		if err == nil {
			profile.BusOwner = owner
		}
	}

	return profile, nil
}

// UpdateStaffProfile updates staff profile fields
func (s *StaffService) UpdateStaffProfile(userID string, updates map[string]interface{}) error {
	// Get existing staff record
	_, err := s.staffRepo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("staff not found: %v", err)
	}

	// Build update fields
	fields := make(map[string]interface{})

	// Handle date fields
	if expiryDate, ok := updates["license_expiry_date"].(string); ok {
		parsedDate, err := time.Parse("2006-01-02", expiryDate)
		if err == nil {
			fields["license_expiry_date"] = parsedDate
		}
	}

	// Handle name fields
	if firstName, ok := updates["first_name"].(string); ok {
		fields["first_name"] = firstName
	}

	if lastName, ok := updates["last_name"].(string); ok {
		fields["last_name"] = lastName
	}

	// Handle other fields
	if licenseNum, ok := updates["license_number"].(string); ok {
		fields["license_number"] = licenseNum
	}

	if expYears, ok := updates["experience_years"].(int); ok {
		fields["experience_years"] = expYears
	}

	if emergContact, ok := updates["emergency_contact"].(string); ok {
		fields["emergency_contact"] = emergContact
	}

	if emergName, ok := updates["emergency_contact_name"].(string); ok {
		fields["emergency_contact_name"] = emergName
	}

	if len(fields) == 0 {
		return fmt.Errorf("no valid fields to update")
	}

	// Update staff record
	err = s.staffRepo.UpdateFields(userID, fields)
	if err != nil {
		return fmt.Errorf("failed to update staff profile: %v", err)
	}

	return nil
}

// CheckStaffRegistration checks if user is registered as staff
func (s *StaffService) CheckStaffRegistration(phoneNumber string) (map[string]interface{}, error) {
	// Get user by phone
	user, err := s.userRepo.GetUserByPhone(phoneNumber)
	if err != nil {
		return map[string]interface{}{
			"is_registered": false,
			"error":         "user_not_found",
		}, nil
	}

	// Check if registered as staff
	staff, err := s.staffRepo.GetByUserID(user.ID.String())
	if err != nil {
		// Not registered as staff
		return map[string]interface{}{
			"is_registered":         false,
			"user_id":               user.ID.String(),
			"requires_registration": true,
		}, nil
	}

	// User is registered as staff - check current employment
	result := map[string]interface{}{
		"is_registered":     true,
		"user_id":           user.ID.String(),
		"staff_id":          staff.ID,
		"staff_type":        staff.StaffType,
		"profile_completed": staff.ProfileCompleted,
		"first_name":        staff.FirstName,
		"last_name":         staff.LastName,
	}

	// Add current employment info if assigned
	employment, err := s.staffRepo.GetCurrentEmployment(staff.ID)
	if err == nil && employment != nil {
		result["employment_status"] = employment.EmploymentStatus
		result["is_employed"] = true
		result["hire_date"] = employment.HireDate

		owner, err := s.ownerRepo.GetByID(employment.BusOwnerID)
		if err == nil {
			result["bus_owner"] = map[string]interface{}{
				"id":           owner.ID,
				"company_name": owner.CompanyName,
			}
		}
	} else {
		result["is_employed"] = false
		result["employment_status"] = "unassigned"
	}

	if !staff.ProfileCompleted {
		result["requires_profile_completion"] = true
	}

	return result, nil
}

// GetBusOwnerByID retrieves bus owner by ID
func (s *StaffService) GetBusOwnerByID(ownerID string) (*models.BusOwner, error) {
	return s.ownerRepo.GetByID(ownerID)
}

// FindBusOwnerByCode searches for bus owner by license code
func (s *StaffService) FindBusOwnerByCode(code string) (*models.BusOwnerPublicInfo, error) {
	owner, err := s.ownerRepo.GetByLicenseNumber(code)
	if err != nil {
		return nil, err
	}

	return &models.BusOwnerPublicInfo{
		ID:                 owner.ID,
		CompanyName:        owner.CompanyName,
		ContactPerson:      owner.ContactPerson,
		City:               owner.City,
		VerificationStatus: owner.VerificationStatus,
		TotalBuses:         owner.TotalBuses,
	}, nil
}

// FindBusOwnerByBusNumber searches for bus owner by bus registration number
// TODO: This requires a buses table and query logic
func (s *StaffService) FindBusOwnerByBusNumber(busNumber string) (*models.BusOwnerPublicInfo, error) {
	// Placeholder: To be implemented when buses table exists
	return nil, fmt.Errorf("bus number search not yet implemented")
}

// AssignBusOwner assigns a bus owner to a staff member (creates new employment record)
func (s *StaffService) AssignBusOwner(userID, busOwnerID string) error {
	// Verify staff exists
	staff, err := s.staffRepo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("staff not found: %v", err)
	}

	// Verify bus owner exists and is verified
	owner, err := s.ownerRepo.GetByID(busOwnerID)
	if err != nil {
		return fmt.Errorf("bus owner not found: %v", err)
	}

	if owner.VerificationStatus != models.VerificationVerified {
		return fmt.Errorf("bus owner is not verified")
	}

	// Check if staff already has current employment
	existingEmployment, _ := s.staffRepo.GetCurrentEmployment(staff.ID)
	if existingEmployment != nil {
		return fmt.Errorf("staff already has active employment with another company")
	}

	// Create new employment record
	now := time.Now()
	employment := &models.BusStaffEmployment{
		StaffID:          staff.ID,
		BusOwnerID:       busOwnerID,
		EmploymentStatus: models.EmploymentStatusActive,
		HireDate:         &now,
		IsCurrent:        true,
	}

	err = s.staffRepo.CreateEmployment(employment)
	if err != nil {
		return fmt.Errorf("failed to create employment record: %v", err)
	}

	return nil
}

// LinkStaffToBusOwner links staff to bus owner (used by bus owner to add staff)
func (s *StaffService) LinkStaffToBusOwner(staffID, busOwnerID string) error {
	// Verify staff exists
	staff, err := s.staffRepo.GetByID(staffID)
	if err != nil {
		return fmt.Errorf("staff not found: %v", err)
	}

	// Verify bus owner exists
	_, err = s.ownerRepo.GetByID(busOwnerID)
	if err != nil {
		return fmt.Errorf("bus owner not found: %v", err)
	}

	// Check if staff already has current employment
	existingEmployment, _ := s.staffRepo.GetCurrentEmployment(staff.ID)
	if existingEmployment != nil {
		return fmt.Errorf("staff already has active employment")
	}

	// Create new employment record
	now := time.Now()
	employment := &models.BusStaffEmployment{
		StaffID:          staff.ID,
		BusOwnerID:       busOwnerID,
		EmploymentStatus: models.EmploymentStatusActive,
		HireDate:         &now,
		IsCurrent:        true,
	}

	err = s.staffRepo.CreateEmployment(employment)
	if err != nil {
		return fmt.Errorf("failed to link staff: %v", err)
	}

	return nil
}

// UnlinkStaff ends the employment of a staff member with a bus owner
func (s *StaffService) UnlinkStaff(staffID, busOwnerID, reason string) error {
	// Verify staff exists
	_, err := s.staffRepo.GetByID(staffID)
	if err != nil {
		return fmt.Errorf("staff not found: %v", err)
	}

	// Get current employment
	employment, err := s.staffRepo.GetCurrentEmployment(staffID)
	if err != nil || employment == nil {
		return fmt.Errorf("no active employment found")
	}

	// Verify the employment is with the correct bus owner
	if employment.BusOwnerID != busOwnerID {
		return fmt.Errorf("staff is not employed by this bus owner")
	}

	// End the employment
	err = s.staffRepo.EndEmployment(staffID, models.EmploymentStatusTerminated, reason)
	if err != nil {
		return fmt.Errorf("failed to end employment: %v", err)
	}

	return nil
}

// GetEmploymentHistory gets all employment history for a staff member
func (s *StaffService) GetEmploymentHistory(staffID string) ([]*models.BusStaffEmployment, error) {
	return s.staffRepo.GetEmploymentHistory(staffID)
}

// GetStaffByBusOwner gets all current staff for a bus owner
func (s *StaffService) GetStaffByBusOwner(busOwnerID string) ([]*models.StaffWithEmployment, error) {
	return s.staffRepo.GetAllByBusOwner(busOwnerID)
}

// ApproveStaff approves a pending staff registration (admin verification)
func (s *StaffService) ApproveStaff(staffID, adminUserID string) error {
	staff, err := s.staffRepo.GetByID(staffID)
	if err != nil {
		return fmt.Errorf("staff not found: %v", err)
	}

	// Update verification fields
	now := time.Now()
	staff.VerifiedAt = &now
	staff.VerifiedBy = &adminUserID
	staff.BackgroundCheckStatus = models.BackgroundCheckApproved

	err = s.staffRepo.Update(staff)
	if err != nil {
		return fmt.Errorf("failed to approve staff: %v", err)
	}

	return nil
}

// stringPtr is a helper to create a pointer to a string
func stringPtr(s string) *string {
	return &s
}
