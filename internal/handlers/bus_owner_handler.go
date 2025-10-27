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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Count permits
	permitCount, err := h.permitRepo.CountPermits(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count permits"})
		return
	}

	// Check if company info is complete
	hasCompanyInfo := busOwner.CompanyName != nil &&
	                  busOwner.IdentityOrIncorporationNo != nil &&
	                  *busOwner.CompanyName != "" &&
	                  *busOwner.IdentityOrIncorporationNo != ""

	c.JSON(http.StatusOK, gin.H{
		"profile_completed": busOwner.ProfileCompleted,
		"permit_count":      permitCount,
		"has_company_info":  hasCompanyInfo,
	})
}

// CompleteOnboardingRequest represents the onboarding request payload
type CompleteOnboardingRequest struct {
	CompanyName              string                              `json:"company_name" binding:"required"`
	IdentityOrIncorporationNo string                             `json:"identity_or_incorporation_no" binding:"required"`
	BusinessEmail            *string                             `json:"business_email,omitempty"`
	Permits                  []models.CreateRoutePermitRequest   `json:"permits" binding:"required,min=1,dive"`
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
				"code": "PROFILE_ALREADY_COMPLETED",
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
		fmt.Printf("DEBUG: Updating user profile - ID: %s, FirstName: %s, LastName: %s\n", newUser.ID, req.FirstName, req.LastName)
		err = h.userRepo.UpdateProfile(newUser.ID, req.FirstName, req.LastName, "", "", "", "")
		if err != nil {
			// Log but don't fail - user is created, name can be updated on first login
			fmt.Printf("WARNING: Failed to update user name: %v\n", err)
		} else {
			fmt.Printf("DEBUG: Successfully updated user name for user %s\n", newUser.ID)

			// Verify the update by re-fetching the user
			updatedUser, err := h.userRepo.GetUserByID(newUser.ID)
			if err == nil {
				fmt.Printf("DEBUG: User after update - FirstName: %s, LastName: %s\n", updatedUser.FirstName.String, updatedUser.LastName.String)
			}
		}

		userID = newUser.ID
	} else {
		// User exists - check if already registered as staff
		existingStaff, _ := h.staffRepo.GetByUserID(existingUser.ID.String())
		if existingStaff != nil {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("This phone number is already registered as %s", existingStaff.StaffType),
				"staff_type": existingStaff.StaffType,
			})
			return
		}

		userID = existingUser.ID
	}

	// Create bus_staff record
	now := time.Now()
	staff := &models.BusStaff{
		UserID:                userID.String(),
		BusOwnerID:            &busOwner.ID,
		StaffType:             req.StaffType,
		LicenseNumber:         &req.NTCLicenseNumber,
		LicenseExpiryDate:     &expiryDate,
		ExperienceYears:       req.ExperienceYears,
		EmploymentStatus:      models.EmploymentStatusActive, // Pre-approved by bus owner
		BackgroundCheckStatus: models.BackgroundCheckPending,
		HireDate:              &now,
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

	// Add role to user (driver or conductor)
	roleToAdd := string(req.StaffType)
	err = h.userRepo.AddUserRole(userID, roleToAdd)
	if err != nil {
		// Log but don't fail - staff record is created
		fmt.Printf("WARNING: Failed to add role %s to user %s: %v\n", roleToAdd, userID, err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    fmt.Sprintf("%s added successfully", req.StaffType),
		"user_id":    userID.String(),
		"staff_id":   staff.ID,
		"staff_type": staff.StaffType,
		"instructions": fmt.Sprintf("Staff member can now login using phone number %s", req.PhoneNumber),
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

	// Get all staff for this bus owner
	staffList, err := h.staffRepo.GetAllByBusOwner(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get staff: %v", err)})
		return
	}

	// Enrich staff data with user information (name, phone)
	type StaffWithUserInfo struct {
		ID                       string                       `json:"id"`
		UserID                   string                       `json:"user_id"`
		FirstName                string                       `json:"first_name"`
		LastName                 string                       `json:"last_name"`
		Phone                    string                       `json:"phone"`
		StaffType                models.StaffType             `json:"staff_type"`
		LicenseNumber            *string                      `json:"license_number,omitempty"`
		LicenseExpiryDate        *time.Time                   `json:"license_expiry_date,omitempty"`
		ExperienceYears          int                          `json:"experience_years"`
		EmergencyContact         *string                      `json:"emergency_contact,omitempty"`
		EmergencyContactName     *string                      `json:"emergency_contact_name,omitempty"`
		EmploymentStatus         models.EmploymentStatus      `json:"employment_status"`
		BackgroundCheckStatus    models.BackgroundCheckStatus `json:"background_check_status"`
		HireDate                 *time.Time                   `json:"hire_date,omitempty"`
		PerformanceRating        float64                      `json:"performance_rating"`
		TotalTripsCompleted      int                          `json:"total_trips_completed"`
		ProfileCompleted         bool                         `json:"profile_completed"`
		CreatedAt                time.Time                    `json:"created_at"`
	}

	enrichedStaff := []StaffWithUserInfo{}
	for _, staff := range staffList {
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
			EmploymentStatus:      staff.EmploymentStatus,
			BackgroundCheckStatus: staff.BackgroundCheckStatus,
			HireDate:              staff.HireDate,
			PerformanceRating:     staff.PerformanceRating,
			TotalTripsCompleted:   staff.TotalTripsCompleted,
			ProfileCompleted:      staff.ProfileCompleted,
			CreatedAt:             staff.CreatedAt,
		}

		enrichedStaff = append(enrichedStaff, enriched)
	}

	c.JSON(http.StatusOK, gin.H{
		"staff": enrichedStaff,
		"total": len(enrichedStaff),
	})
}
