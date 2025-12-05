package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type BusOwnerHandler struct {
	busOwnerRepo *database.BusOwnerRepository
	permitRepo   *database.RoutePermitRepository
	userRepo     *database.UserRepository
	staffRepo    *database.BusStaffRepository
}

func NewBusOwnerHandler(busOwnerRepo *database.BusOwnerRepository, permitRepo *database.RoutePermitRepository, userRepo *database.UserRepository, staffRepo *database.BusStaffRepository) *BusOwnerHandler {
	return &BusOwnerHandler{
		busOwnerRepo: busOwnerRepo,
		permitRepo:   permitRepo,
		userRepo:     userRepo,
		staffRepo:    staffRepo,
	}
}

// GetProfile retrieves the bus owner profile
// GET /api/v1/bus-owner/profile
func (h *BusOwnerHandler) GetProfile(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	c.JSON(http.StatusOK, busOwner)
}

// CheckProfileStatus checks if bus owner has completed onboarding
// GET /api/v1/bus-owner/profile-status
func (h *BusOwnerHandler) CheckProfileStatus(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner by user_id
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		// Log the error for debugging
		fmt.Printf("ERROR: Failed to fetch bus owner profile for user_id=%s: %v\n", userCtx.UserID.String(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch profile",
			"details": err.Error(),
		})
		return
	}

	// Count permits (non-critical - default to 0 on error)
	permitCount, err := h.permitRepo.CountPermits(busOwner.ID)
	if err != nil {
		// Log the error but don't fail the entire profile request
		// User can still access the app without permit count
		permitCount = 0
	}

	// Check if company info is complete
	hasCompanyInfo := busOwner.CompanyName != nil &&
		busOwner.IdentityOrIncorporationNo != nil &&
		*busOwner.CompanyName != "" &&
		*busOwner.IdentityOrIncorporationNo != ""

	c.JSON(http.StatusOK, gin.H{
		"user_id":           userCtx.UserID.String(),
		"phone":             userCtx.Phone,
		"profile_completed": busOwner.ProfileCompleted,
		"permit_count":      permitCount,
		"has_company_info":  hasCompanyInfo,
	})
}

// CompleteOnboardingRequest represents the onboarding request payload
type CompleteOnboardingRequest struct {
	CompanyName               string                            `json:"company_name" binding:"required"`
	IdentityOrIncorporationNo string                            `json:"identity_or_incorporation_no" binding:"required"`
	BusinessEmail             *string                           `json:"business_email,omitempty"`
	Permits                   []models.CreateRoutePermitRequest `json:"permits" binding:"required,min=1,dive"`
}

// CompleteOnboarding handles the complete onboarding process
// POST /api/v1/bus-owner/complete-onboarding
func (h *BusOwnerHandler) CompleteOnboarding(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request first (need company_name for bus_owner creation)
	var req CompleteOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate at least one permit
	if len(req.Permits) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one permit is required"})
		return
	}

	// Get or create bus owner record (with company_name)
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			// Bus owner record doesn't exist, create it with company info
			busOwner, err = h.busOwnerRepo.CreateWithCompany(
				userCtx.UserID.String(),
				req.CompanyName,
				req.IdentityOrIncorporationNo,
				req.BusinessEmail,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bus owner profile"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
			return
		}
	} else {
		// Bus owner exists - check if profile is already completed
		if busOwner.ProfileCompleted {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Profile already completed. Onboarding can only be done once.",
				"code":  "PROFILE_ALREADY_COMPLETED",
			})
			return
		}

		// Profile exists but not completed - update company info
		err = h.busOwnerRepo.UpdateProfile(
			busOwner.ID,
			req.CompanyName,
			req.IdentityOrIncorporationNo,
			req.BusinessEmail,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}
	}

	// Create permits (trigger will auto-set profile_completed)
	createdPermits := make([]models.RoutePermit, 0, len(req.Permits))
	for _, permitReq := range req.Permits {
		permit, err := models.NewRoutePermitFromRequest(busOwner.ID, &permitReq)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = h.permitRepo.Create(permit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create permit: " + err.Error()})
			return
		}

		createdPermits = append(createdPermits, *permit)
	}

	// Update users table to mark profile as completed
	// NOTE: bus_owners.profile_completed is automatically set by database trigger,
	// but we need to also update users.profile_completed for consistency
	err = h.userRepo.SetProfileCompleted(userCtx.UserID, true)
	if err != nil {
		// Log error but don't fail the request - bus owner profile is already complete
		// In production, you'd log this properly
	}

	// Add "bus_owner" role to user's roles array
	// This ensures JWT tokens include the role for authorization
	err = h.userRepo.AddUserRole(userCtx.UserID, "bus_owner")
	if err != nil {
		// Log error but don't fail the request
		// Role might already exist (AddUserRole prevents duplicates)
	}

	// Fetch updated profile (should have profile_completed = true now)
	updatedProfile, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Onboarding completed successfully",
		"profile": updatedProfile,
		"permits": createdPermits,
	})
}

// VerifyStaff checks if a staff member can be added by bus owner
// POST /api/v1/bus-owner/staff/verify
func (h *BusOwnerHandler) VerifyStaff(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner record
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found. Please complete onboarding first."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Parse request
	var req models.VerifyStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("DEBUG: VerifyStaff - Checking phone: %s\n", req.PhoneNumber)

	// Check if user exists by phone number
	existingUser, err := h.userRepo.GetUserByPhone(req.PhoneNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user existence"})
		return
	}

	if existingUser == nil {
		// User not found in database
		c.JSON(http.StatusOK, &models.VerifyStaffResponse{
			Found:            false,
			Eligible:         false,
			ProfileCompleted: false,
			IsVerified:       false,
			AlreadyLinked:    false,
			Message:          "This phone number is not registered in our system",
			Reason:           "not_registered",
		})
		return
	}

	// Check if user is registered as staff
	staff, err := h.staffRepo.GetByUserID(existingUser.ID.String())
	if err != nil || staff == nil {
		// User exists but not registered as staff
		c.JSON(http.StatusOK, &models.VerifyStaffResponse{
			Found:            true,
			Eligible:         false,
			FirstName:        existingUser.FirstName.String,
			LastName:         existingUser.LastName.String,
			ProfileCompleted: false,
			IsVerified:       false,
			AlreadyLinked:    false,
			Message:          "This user has not registered as a driver or conductor",
			Reason:           "not_staff",
		})
		return
	}

	// Check if profile is completed
	if !staff.ProfileCompleted {
		c.JSON(http.StatusOK, &models.VerifyStaffResponse{
			Found:            true,
			Eligible:         false,
			StaffID:          staff.ID,
			StaffType:        &staff.StaffType,
			FirstName:        existingUser.FirstName.String,
			LastName:         existingUser.LastName.String,
			ProfileCompleted: false,
			IsVerified:       false,
			AlreadyLinked:    false,
			Message:          "This staff member has not completed their profile",
			Reason:           "profile_incomplete",
		})
		return
	}

	// Check if verified by admin (verified_at is not null)
	isVerified := staff.VerifiedAt != nil
	if !isVerified {
		c.JSON(http.StatusOK, &models.VerifyStaffResponse{
			Found:            true,
			Eligible:         false,
			StaffID:          staff.ID,
			StaffType:        &staff.StaffType,
			FirstName:        existingUser.FirstName.String,
			LastName:         existingUser.LastName.String,
			ProfileCompleted: true,
			IsVerified:       false,
			AlreadyLinked:    false,
			Message:          "This staff member is pending admin verification",
			Reason:           "not_verified",
		})
		return
	}

	// Check if already has active employment (via bus_staff_employment table)
	currentEmployment, _ := h.staffRepo.GetCurrentEmployment(staff.ID)
	if currentEmployment != nil {
		if currentEmployment.BusOwnerID != busOwner.ID {
			c.JSON(http.StatusOK, &models.VerifyStaffResponse{
				Found:            true,
				Eligible:         false,
				StaffID:          staff.ID,
				StaffType:        &staff.StaffType,
				FirstName:        existingUser.FirstName.String,
				LastName:         existingUser.LastName.String,
				ProfileCompleted: true,
				IsVerified:       true,
				AlreadyLinked:    true,
				CurrentOwnerID:   &currentEmployment.BusOwnerID,
				Message:          "This staff member is already employed by another bus owner",
				Reason:           "already_linked_other",
			})
			return
		} else {
			// Already linked to this bus owner
			c.JSON(http.StatusOK, &models.VerifyStaffResponse{
				Found:            true,
				Eligible:         false,
				StaffID:          staff.ID,
				StaffType:        &staff.StaffType,
				FirstName:        existingUser.FirstName.String,
				LastName:         existingUser.LastName.String,
				ProfileCompleted: true,
				IsVerified:       true,
				AlreadyLinked:    true,
				CurrentOwnerID:   &currentEmployment.BusOwnerID,
				Message:          "This staff member is already part of your organization",
				Reason:           "already_yours",
			})
			return
		}
	}

	// Staff is eligible to be added
	c.JSON(http.StatusOK, &models.VerifyStaffResponse{
		Found:            true,
		Eligible:         true,
		StaffID:          staff.ID,
		StaffType:        &staff.StaffType,
		FirstName:        existingUser.FirstName.String,
		LastName:         existingUser.LastName.String,
		ProfileCompleted: true,
		IsVerified:       true,
		AlreadyLinked:    false,
		Message:          fmt.Sprintf("Staff member %s %s is verified and can be added to your organization", existingUser.FirstName.String, existingUser.LastName.String),
	})
}

// LinkStaff links a verified staff member to the bus owner's organization
// POST /api/v1/bus-owner/staff/link
func (h *BusOwnerHandler) LinkStaff(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner record
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found. Please complete onboarding first."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Parse request
	var req models.LinkStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("DEBUG: LinkStaff - Linking staff %s to bus owner %s\n", req.StaffID, busOwner.ID)

	// Get staff by ID
	staff, err := h.staffRepo.GetByID(req.StaffID)
	if err != nil || staff == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Staff member not found"})
		return
	}

	// Verify staff is eligible (re-check for security)
	if !staff.ProfileCompleted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Staff member has not completed their profile"})
		return
	}

	if staff.VerifiedAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Staff member is not verified by admin"})
		return
	}

	// Check for existing employment
	currentEmployment, _ := h.staffRepo.GetCurrentEmployment(staff.ID)
	if currentEmployment != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Staff member already has active employment"})
		return
	}

	// Create employment record to link staff to bus owner
	now := time.Now()
	employment := &models.BusStaffEmployment{
		StaffID:          staff.ID,
		BusOwnerID:       busOwner.ID,
		EmploymentStatus: models.EmploymentStatusActive,
		HireDate:         &now,
		IsCurrent:        true,
	}

	err = h.staffRepo.CreateEmployment(employment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to link staff: %v", err)})
		return
	}

	// Get user info for response
	user, _ := h.userRepo.GetUserByID(uuid.MustParse(staff.UserID))
	firstName := ""
	lastName := ""
	phone := ""
	if user != nil {
		firstName = user.FirstName.String
		lastName = user.LastName.String
		phone = user.Phone
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       fmt.Sprintf("%s %s has been added to your organization", firstName, lastName),
		"staff_id":      staff.ID,
		"employment_id": employment.ID,
		"staff_type":    staff.StaffType,
		"first_name":    firstName,
		"last_name":     lastName,
		"phone":         phone,
		"hire_date":     employment.HireDate,
	})
}

// AddStaff allows bus owner to add driver or conductor to their organization
// POST /api/v1/bus-owner/staff
func (h *BusOwnerHandler) AddStaff(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner record
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found. Please complete onboarding first."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Parse request
	var req models.AddStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// DEBUG: Log received request data
	fmt.Printf("DEBUG: AddStaff request received - Phone: %s, FirstName: '%s', LastName: '%s', Type: %s\n",
		req.PhoneNumber, req.FirstName, req.LastName, req.StaffType)

	// Validate staff type
	if req.StaffType != models.StaffTypeDriver && req.StaffType != models.StaffTypeConductor {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid staff_type. Must be 'driver' or 'conductor'"})
		return
	}

	// Validate and parse license expiry date
	expiryDate, err := time.Parse("2006-01-02", req.LicenseExpiryDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid license_expiry_date format. Use YYYY-MM-DD"})
		return
	}

	// Check if license has expired
	if expiryDate.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "NTC license has already expired"})
		return
	}

	// Check if user exists by phone number
	existingUser, err := h.userRepo.GetUserByPhone(req.PhoneNumber)
	var userID uuid.UUID

	if err != nil {
		// Database error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user existence"})
		return
	}

	if existingUser == nil {
		// User doesn't exist - create new user account
		newUser, err := h.userRepo.CreateUserWithoutRole(req.PhoneNumber)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create user account: %v", err)})
			return
		}

		// Update user's first and last name (since CreateUserWithoutRole doesn't set these)
		fmt.Printf("DEBUG: Updating user names - ID: %s, FirstName: %s, LastName: %s\n", newUser.ID, req.FirstName, req.LastName)
		err = h.userRepo.UpdateUserNames(newUser.ID, req.FirstName, req.LastName)
		if err != nil {
			// This is critical - fail if we can't set the names
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update user names: %v", err)})
			return
		}
		fmt.Printf("DEBUG: Successfully updated user names for user %s\n", newUser.ID)

		// Verify the update by re-fetching the user
		updatedUser, err := h.userRepo.GetUserByID(newUser.ID)
		if err == nil {
			fmt.Printf("DEBUG: User after update - FirstName: '%s', LastName: '%s'\n", updatedUser.FirstName.String, updatedUser.LastName.String)
		}

		userID = newUser.ID
	} else {
		// User exists - check if already registered as staff
		existingStaff, _ := h.staffRepo.GetByUserID(existingUser.ID.String())
		if existingStaff != nil {
			// Check if they have active employment
			currentEmployment, _ := h.staffRepo.GetCurrentEmployment(existingStaff.ID)
			if currentEmployment != nil {
				c.JSON(http.StatusConflict, gin.H{
					"error":      fmt.Sprintf("This phone number is already employed as %s", existingStaff.StaffType),
					"staff_type": existingStaff.StaffType,
				})
				return
			}
			// Staff exists but not employed - link them instead
			now := time.Now()
			employment := &models.BusStaffEmployment{
				StaffID:          existingStaff.ID,
				BusOwnerID:       busOwner.ID,
				EmploymentStatus: models.EmploymentStatusActive,
				HireDate:         &now,
				IsCurrent:        true,
			}
			err = h.staffRepo.CreateEmployment(employment)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to link existing staff: %v", err)})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message":       fmt.Sprintf("Existing %s linked to your organization", existingStaff.StaffType),
				"staff_id":      existingStaff.ID,
				"employment_id": employment.ID,
				"staff_type":    existingStaff.StaffType,
			})
			return
		}

		userID = existingUser.ID
	}

	// Create bus_staff record (profile only)
	staff := &models.BusStaff{
		UserID:                userID.String(),
		FirstName:             &req.FirstName,
		LastName:              &req.LastName,
		StaffType:             req.StaffType,
		LicenseNumber:         &req.NTCLicenseNumber,
		LicenseExpiryDate:     &expiryDate,
		ExperienceYears:       req.ExperienceYears,
		BackgroundCheckStatus: models.BackgroundCheckPending,
		ProfileCompleted:      true, // Profile is complete since bus owner provided all info
	}

	if req.EmergencyContact != "" {
		staff.EmergencyContact = &req.EmergencyContact
	}
	if req.EmergencyContactName != "" {
		staff.EmergencyContactName = &req.EmergencyContactName
	}

	// Create staff record in database
	err = h.staffRepo.Create(staff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create staff record: %v", err)})
		return
	}

	// Create employment record to link staff to this bus owner
	now := time.Now()
	employment := &models.BusStaffEmployment{
		StaffID:          staff.ID,
		BusOwnerID:       busOwner.ID,
		EmploymentStatus: models.EmploymentStatusActive,
		HireDate:         &now,
		IsCurrent:        true,
	}

	err = h.staffRepo.CreateEmployment(employment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create employment record: %v", err)})
		return
	}

	// Add role to user (driver or conductor)
	roleToAdd := string(req.StaffType)
	err = h.userRepo.AddUserRole(userID, roleToAdd)
	if err != nil {
		// Log but don't fail - staff record is created
		fmt.Printf("WARNING: Failed to add role %s to user %s: %v\n", roleToAdd, userID, err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       fmt.Sprintf("%s added successfully", req.StaffType),
		"user_id":       userID.String(),
		"staff_id":      staff.ID,
		"employment_id": employment.ID,
		"staff_type":    staff.StaffType,
		"hire_date":     employment.HireDate,
		"instructions":  fmt.Sprintf("Staff member can now login using phone number %s", req.PhoneNumber),
	})
}

// GetStaff retrieves all staff (drivers and conductors) for the authenticated bus owner
// GET /api/v1/bus-owner/staff
func (h *BusOwnerHandler) GetStaff(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner record
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Get all current staff for this bus owner (via employment table)
	staffWithEmployment, err := h.staffRepo.GetAllByBusOwner(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get staff: %v", err)})
		return
	}

	// Enrich staff data with user information (name, phone)
	type StaffWithUserInfo struct {
		ID                    string                  `json:"id"`
		UserID                string                  `json:"user_id"`
		FirstName             string                  `json:"first_name"`
		LastName              string                        `json:"last_name"`
		Phone                 string                        `json:"phone"`
		StaffType             models.StaffType              `json:"staff_type"`
		LicenseNumber         *string                       `json:"license_number,omitempty"`
		LicenseExpiryDate     *time.Time                    `json:"license_expiry_date,omitempty"`
		ExperienceYears       int                           `json:"experience_years"`
		EmergencyContact      *string                       `json:"emergency_contact,omitempty"`
		EmergencyContactName  *string                       `json:"emergency_contact_name,omitempty"`
		EmploymentStatus      models.EmploymentStatus       `json:"employment_status"`
		BackgroundCheckStatus models.BackgroundCheckStatus  `json:"background_check_status"`
		HireDate              *time.Time                    `json:"hire_date,omitempty"`
		PerformanceRating     float64                       `json:"performance_rating"`
		TotalTripsCompleted   int                           `json:"total_trips_completed"`
		ProfileCompleted      bool                          `json:"profile_completed"`
		EmploymentID          string                        `json:"employment_id"`
		CreatedAt             time.Time                     `json:"created_at"`
	}

	enrichedStaff := []StaffWithUserInfo{}
	for _, swe := range staffWithEmployment {
		staff := swe.Staff
		employment := swe.Employment

		// Get user information
		user, err := h.userRepo.GetUserByID(uuid.MustParse(staff.UserID))
		if err != nil {
			// Log error but don't fail the whole request
			fmt.Printf("WARNING: Failed to get user info for staff %s: %v\n", staff.ID, err)
			continue
		}

		fmt.Printf("DEBUG: GetStaff - User ID: %s, FirstName: '%s', LastName: '%s', Phone: %s\n",
			user.ID, user.FirstName.String, user.LastName.String, user.Phone)

		enriched := StaffWithUserInfo{
			ID:                    staff.ID,
			UserID:                staff.UserID,
			FirstName:             user.FirstName.String,
			LastName:              user.LastName.String,
			Phone:                 user.Phone,
			StaffType:             staff.StaffType,
			LicenseNumber:         staff.LicenseNumber,
			LicenseExpiryDate:     staff.LicenseExpiryDate,
			ExperienceYears:       staff.ExperienceYears,
			EmergencyContact:      staff.EmergencyContact,
			EmergencyContactName:  staff.EmergencyContactName,
			EmploymentStatus:      employment.EmploymentStatus,
			BackgroundCheckStatus: staff.BackgroundCheckStatus,
			HireDate:              employment.HireDate,
			PerformanceRating:     employment.PerformanceRating,
			TotalTripsCompleted:   employment.TotalTripsCompleted,
			ProfileCompleted:      staff.ProfileCompleted,
			EmploymentID:          employment.ID,
			CreatedAt:             staff.CreatedAt,
		}

		enrichedStaff = append(enrichedStaff, enriched)
	}

	c.JSON(http.StatusOK, gin.H{
		"staff": enrichedStaff,
		"total": len(enrichedStaff),
	})
}

// UnlinkStaff removes a staff member from the bus owner's organization
// POST /api/v1/bus-owner/staff/unlink
func (h *BusOwnerHandler) UnlinkStaff(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get bus owner record
	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get bus owner profile"})
		return
	}

	// Parse request
	var req models.UnlinkStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify staff exists and is employed by this bus owner
	currentEmployment, err := h.staffRepo.GetCurrentEmployment(req.StaffID)
	if err != nil || currentEmployment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active employment found for this staff member"})
		return
	}

	if currentEmployment.BusOwnerID != busOwner.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "This staff member is not employed by your organization"})
		return
	}

	// Determine status based on request (default to terminated)
	status := models.EmploymentStatusTerminated
	if req.Status == "resigned" {
		status = models.EmploymentStatusResigned
	}

	// End the employment
	err = h.staffRepo.EndEmployment(req.StaffID, status, req.TerminationReason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unlink staff: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Staff member has been removed from your organization",
		"staff_id": req.StaffID,
	})
}
