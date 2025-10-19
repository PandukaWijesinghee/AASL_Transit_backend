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

// RegisterStaff registers a new driver or conductor
func (s *StaffService) RegisterStaff(input *models.StaffRegistrationInput) (*models.BusStaff, error) {
	// Validate staff type
	if input.StaffType != models.StaffTypeDriver && input.StaffType != models.StaffTypeConductor {
		return nil, fmt.Errorf("invalid staff type")
	}

	// Check if user exists
	// Convert string ID to UUID
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

	// Build staff record
	staff := &models.BusStaff{
		UserID:               input.UserID,
		StaffType:            input.StaffType,
		ExperienceYears:      input.ExperienceYears,
		EmergencyContact:     &input.EmergencyContact,
		EmergencyContactName: &input.EmergencyContactName,
		EmploymentStatus:     models.EmploymentStatusPending,
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

	// Handle bus owner assignment (optional)
	if input.BusOwnerCode != nil && *input.BusOwnerCode != "" {
		owner, err := s.ownerRepo.GetByLicenseNumber(*input.BusOwnerCode)
		if err == nil {
			staff.BusOwnerID = &owner.ID
		}
	} else if input.BusRegistrationNumber != nil && *input.BusRegistrationNumber != "" {
		// TODO: Implement bus search by registration number if needed
		// For now, bus_owner_id will remain NULL
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

// GetCompleteProfile retrieves complete staff profile with user and bus owner info
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

	// Get bus owner if assigned
	if staff.BusOwnerID != nil {
		owner, err := s.ownerRepo.GetByID(*staff.BusOwnerID)
		if err == nil {
			profile.BusOwner = owner
		}
	}

	return profile, nil
}

// UpdateStaffProfile updates staff profile fields
func (s *StaffService) UpdateStaffProfile(userID string, updates map[string]interface{}) error {
	// Get existing staff record
	staff, err := s.staffRepo.GetByUserID(userID)
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

	if medCertExpiry, ok := updates["medical_certificate_expiry"].(string); ok {
		parsedDate, err := time.Parse("2006-01-02", medCertExpiry)
		if err == nil {
			fields["medical_certificate_expiry"] = parsedDate
		}
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
	err = s.staffRepo.UpdateFields(staff.UserID, fields)
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

	// User is registered as staff
	result := map[string]interface{}{
		"is_registered":     true,
		"user_id":           user.ID.String(),
		"staff_id":          staff.ID,
		"staff_type":        staff.StaffType,
		"profile_completed": staff.ProfileCompleted,
		"employment_status": staff.EmploymentStatus,
	}

	// Add bus owner info if assigned
	if staff.BusOwnerID != nil {
		owner, err := s.ownerRepo.GetByID(*staff.BusOwnerID)
		if err == nil {
			result["bus_owner"] = map[string]interface{}{
				"id":           owner.ID,
				"company_name": owner.CompanyName,
			}
		}
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

// AssignBusOwner assigns a bus owner to a staff member
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

	// Update staff record
	staff.BusOwnerID = &busOwnerID
	err = s.staffRepo.Update(staff)
	if err != nil {
		return fmt.Errorf("failed to assign bus owner: %v", err)
	}

	return nil
}

// ApproveStaff approves a pending staff registration
func (s *StaffService) ApproveStaff(staffID, adminUserID string) error {
	staff, err := s.staffRepo.GetByID(staffID)
	if err != nil {
		return fmt.Errorf("staff not found: %v", err)
	}

	if staff.EmploymentStatus != models.EmploymentStatusPending {
		return fmt.Errorf("staff is not in pending status")
	}

	// Update to active status
	staff.EmploymentStatus = models.EmploymentStatusActive
	now := time.Now()
	staff.HireDate = &now
	staff.VerifiedAt = &now
	staff.VerifiedBy = &adminUserID

	err = s.staffRepo.Update(staff)
	if err != nil {
		return fmt.Errorf("failed to approve staff: %v", err)
	}

	return nil
}
