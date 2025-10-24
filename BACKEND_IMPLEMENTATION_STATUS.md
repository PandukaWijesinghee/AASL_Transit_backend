# Backend Implementation Status - Bus Owner Onboarding

## ✅ COMPLETED

### 1. Database Migration
**File:** `migrations/001_bus_owner_onboarding.sql`
- ✅ Updated `bus_owners` table with `profile_completed` and `identity_or_incorporation_no` fields
- ✅ Created `route_permits` table with all required fields
- ✅ Created `route_permit_stops` table for predefined stops
- ✅ Added helper functions for permit validation
- ✅ Added trigger to auto-update `profile_completed` when first permit added
- ✅ Created indexes for performance
- ✅ Included sample data for testing (commented out)

**To Apply:**
```bash
psql -U postgres -d smart_transit -f migrations/001_bus_owner_onboarding.sql
```

### 2. Data Models
**Files:**
- ✅ `internal/models/bus_owner.go` - Updated with new fields
- ✅ `internal/models/route_permit.go` - Complete permit model with validation

**Key Changes:**
- BusOwner: Added `IdentityOrIncorporationNo` and `ProfileCompleted` fields
- Created RoutePermit model with:
  - Bus registration number
  - Via stops array
  - Approved fare
  - Validity dates
  - Methods: `IsValid()`, `IsExpiringSoon()`, `DaysUntilExpiry()`

### 3. Repository Layer
**Files:**
- ✅ `internal/database/route_permit_repository.go` - Complete CRUD operations

**Methods Implemented:**
- `Create()` - Add new permit
- `GetByID()` - Retrieve permit by ID
- `GetByOwnerID()` - Get all permits for an owner
- `GetByPermitNumber()` - Find permit by number
- `GetByBusRegistration()` - Find permit by license plate
- `Update()` - Update permit fields
- `Delete()` - Delete permit
- `GetValidPermits()` - Get valid, non-expired permits
- `CountPermits()` - Count permits for owner

---

## 🔨 TODO: Remaining Backend Work

### 1. Update BusOwnerRepository
**File:** `internal/database/bus_owner_repository.go`

**Changes Needed:**
- Update all SELECT queries to include `profile_completed` and `identity_or_incorporation_no`
- Remove references to `license_number`
- Add method `Create(owner *models.BusOwner)` for registration
- Add method `UpdateProfile(ownerID string, fields map[string]interface{})`

**Example Query Update:**
```go
// OLD:
SELECT id, user_id, company_name, license_number, ...

// NEW:
SELECT id, user_id, company_name, identity_or_incorporation_no, profile_completed, ...
```

### 2. Create Bus Owner Handler
**File:** `internal/handlers/bus_owner_handler.go` (NEW FILE)

**Endpoints to Implement:**

#### A. Complete Onboarding
```go
POST /api/v1/bus-owner/complete-onboarding
Authorization: Bearer <token>

Request Body:
{
  "company_name": "ABC Transport",
  "identity_or_incorporation_no": "PV-2023-1234",
  "business_email": "info@abc.lk"
}

Response:
{
  "success": true,
  "message": "Profile created successfully",
  "bus_owner": { ... },
  "profile_completed": false  // Will be true after first permit added
}
```

**Implementation:**
```go
func (h *BusOwnerHandler) CompleteOnboarding(c *gin.Context) {
    // 1. Get user context from JWT
    userCtx := c.MustGet("user").(middleware.UserContext)

    // 2. Parse request body
    var req CompleteOnboardingRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 3. Check if bus owner already exists
    existing, _ := h.busOwnerRepo.GetByUserID(userCtx.UserID)
    if existing != nil {
        c.JSON(409, gin.H{"error": "Bus owner profile already exists"})
        return
    }

    // 4. Create bus owner record
    owner := &models.BusOwner{
        ID:                        uuid.New().String(),
        UserID:                    userCtx.UserID,
        CompanyName:               req.CompanyName,
        IdentityOrIncorporationNo: &req.IdentityNo,
        BusinessEmail:             &req.BusinessEmail,
        BusinessPhone:             &userCtx.Phone,
        Country:                   "Sri Lanka",
        VerificationStatus:        models.VerificationPending,
        ProfileCompleted:          false, // Will be updated by trigger
    }

    // 5. Save to database
    err := h.busOwnerRepo.Create(owner)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to create profile"})
        return
    }

    // 6. Add "bus_owner" role to user
    err = h.userRepo.AddRole(userCtx.UserID, "bus_owner")
    if err != nil {
        // Log error but don't fail
    }

    // 7. Return success
    c.JSON(201, gin.H{
        "success": true,
        "message": "Profile created successfully",
        "bus_owner": owner,
    })
}
```

#### B. Get Bus Owner Profile
```go
GET /api/v1/bus-owner/profile
Authorization: Bearer <token>

Response:
{
  "success": true,
  "bus_owner": {
    "id": "uuid",
    "company_name": "ABC Transport",
    "profile_completed": true,
    "total_buses": 3,
    "total_permits": 5,
    ...
  }
}
```

### 3. Create Permit Handler
**File:** `internal/handlers/permit_handler.go` (NEW FILE)

**Endpoints to Implement:**

#### A. Add Permit
```go
POST /api/v1/route-permits
Authorization: Bearer <token>

Request Body:
{
  "permit_number": "PERMIT-2025-001",
  "bus_registration_number": "WP CAB-1234",
  "route_number": "138",
  "from_city": "Colombo",
  "to_city": "Kandy",
  "via": "Kadawatha, Kegalle",
  "approved_fare": 250.00,
  "validity_from": "2024-01-01",
  "validity_to": "2025-12-31"
}

Response:
{
  "success": true,
  "message": "Permit added successfully",
  "permit": { ... },
  "profile_completed": true  // If this was first permit
}
```

**Implementation:**
```go
func (h *PermitHandler) CreatePermit(c *gin.Context) {
    // 1. Get bus owner from context
    userCtx := c.MustGet("user").(middleware.UserContext)

    busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID)
    if err != nil || busOwner == nil {
        c.JSON(403, gin.H{
            "error": "PROFILE_INCOMPLETE",
            "message": "Please complete your profile first",
        })
        return
    }

    // 2. Parse request
    var req models.CreateRoutePermitRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 3. Validate request
    if err := req.Validate(); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 4. Check for duplicate permit number
    existing, _ := h.permitRepo.GetByPermitNumber(req.PermitNumber, busOwner.ID)
    if existing != nil {
        c.JSON(409, gin.H{
            "error": "DUPLICATE_PERMIT",
            "message": "Permit number already exists",
        })
        return
    }

    // 5. Parse dates
    issueDate, _ := time.Parse("2006-01-02", req.ValidityFrom)
    expiryDate, _ := time.Parse("2006-01-02", req.ValidityTo)

    // 6. Parse via string into array
    var viaArray models.StringArray
    if req.Via != nil {
        viaStops := strings.Split(*req.Via, ",")
        for i := range viaStops {
            viaStops[i] = strings.TrimSpace(viaStops[i])
        }
        viaArray = models.StringArray(viaStops)
    }

    // 7. Create permit
    permit := &models.RoutePermit{
        ID:                    uuid.New().String(),
        BusOwnerID:            busOwner.ID,
        PermitNumber:          req.PermitNumber,
        BusRegistrationNumber: strings.ToUpper(req.BusRegistrationNumber),
        RouteNumber:           req.RouteNumber,
        RouteName:             fmt.Sprintf("%s - %s", req.FromCity, req.ToCity),
        FullOriginCity:        req.FromCity,
        FullDestinationCity:   req.ToCity,
        Via:                   viaArray,
        TotalDistanceKm:       req.TotalDistanceKm,
        EstimatedDurationMinutes: req.EstimatedDuration,
        IssueDate:             issueDate,
        ExpiryDate:            expiryDate,
        PermitType:            *req.PermitType,
        ApprovedFare:          req.ApprovedFare,
        MaxTripsPerDay:        req.MaxTripsPerDay,
        AllowedBusTypes:       models.StringArray(req.AllowedBusTypes),
        Restrictions:          req.Restrictions,
        Status:                models.VerificationPending,
    }

    // 8. Save to database
    err = h.permitRepo.Create(permit)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to add permit"})
        return
    }

    // 9. Check if profile_completed was updated (by trigger)
    busOwner, _ = h.busOwnerRepo.GetByID(busOwner.ID)

    // 10. Return success
    c.JSON(201, gin.H{
        "success": true,
        "message": "Permit added successfully",
        "permit": permit,
        "profile_completed": busOwner.ProfileCompleted,
    })
}
```

#### B. Get Permits
```go
GET /api/v1/route-permits
Authorization: Bearer <token>

Query Parameters:
- status: pending|verified|expired (optional)
- expired: true|false (optional)

Response:
{
  "success": true,
  "permits": [ ... ],
  "total": 3
}
```

#### C. Get Permit by ID
```go
GET /api/v1/route-permits/:id
Authorization: Bearer <token>

Response:
{
  "success": true,
  "permit": { ... }
}
```

#### D. Update Permit
```go
PUT /api/v1/route-permits/:id
Authorization: Bearer <token>

Request Body:
{
  "bus_registration_number": "WP CAB-1234",
  "via": "Kadawatha, Kegalle, Mawanella",
  "approved_fare": 275.00
}

Response:
{
  "success": true,
  "message": "Permit updated successfully"
}
```

#### E. Delete Permit
```go
DELETE /api/v1/route-permits/:id
Authorization: Bearer <token>

Response:
{
  "success": true,
  "message": "Permit deleted successfully"
}
```

### 4. Update main.go
**File:** `cmd/server/main.go`

**Add Routes:**
```go
// Bus Owner Routes (Protected)
busOwnerRoutes := api.Group("/bus-owner")
busOwnerRoutes.Use(middleware.AuthMiddleware(jwtService))
{
    busOwnerRoutes.POST("/complete-onboarding", busOwnerHandler.CompleteOnboarding)
    busOwnerRoutes.GET("/profile", busOwnerHandler.GetProfile)
    busOwnerRoutes.PUT("/profile", busOwnerHandler.UpdateProfile)
}

// Route Permit Routes (Protected)
permitRoutes := api.Group("/route-permits")
permitRoutes.Use(middleware.AuthMiddleware(jwtService))
permitRoutes.Use(middleware.RequireRole("bus_owner")) // Only bus owners
{
    permitRoutes.POST("", permitHandler.CreatePermit)
    permitRoutes.GET("", permitHandler.GetPermits)
    permitRoutes.GET("/:id", permitHandler.GetPermitByID)
    permitRoutes.PUT("/:id", permitHandler.UpdatePermit)
    permitRoutes.DELETE("/:id", permitHandler.DeletePermit)
}
```

### 5. Create Request/Response Types
**File:** `internal/handlers/types.go` or individual handler files

```go
type CompleteOnboardingRequest struct {
    CompanyName   string  `json:"company_name" binding:"required"`
    IdentityNo    string  `json:"identity_or_incorporation_no" binding:"required"`
    BusinessEmail *string `json:"business_email,omitempty"`
}

type BusOwnerProfileResponse struct {
    Success   bool              `json:"success"`
    BusOwner  *models.BusOwner  `json:"bus_owner"`
    TotalPermits int            `json:"total_permits"`
}

type PermitListResponse struct {
    Success bool                `json:"success"`
    Permits []models.RoutePermit `json:"permits"`
    Total   int                 `json:"total"`
}
```

---

## 📝 Testing Checklist

### Database Setup
- [ ] Apply migration: `psql -d db -f migrations/001_bus_owner_onboarding.sql`
- [ ] Verify tables created: `route_permits`, `route_permit_stops`
- [ ] Verify columns added to `bus_owners`: `profile_completed`, `identity_or_incorporation_no`
- [ ] Test trigger: Insert permit → Check `profile_completed` updates to `true`

### API Testing (Postman/curl)

#### 1. Complete Onboarding
```bash
curl -X POST http://localhost:8080/api/v1/bus-owner/complete-onboarding \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "company_name": "Test Transport",
    "identity_or_incorporation_no": "123456789V",
    "business_email": "test@test.lk"
  }'

# Expected: 201 Created, bus_owner object with profile_completed=false
```

#### 2. Add First Permit
```bash
curl -X POST http://localhost:8080/api/v1/route-permits \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "permit_number": "TEST-001",
    "bus_registration_number": "WP CAB-TEST",
    "route_number": "999",
    "from_city": "Test City A",
    "to_city": "Test City B",
    "via": "City C, City D",
    "approved_fare": 100.00,
    "validity_from": "2024-01-01",
    "validity_to": "2025-12-31"
  }'

# Expected: 201 Created, profile_completed=true in response
```

#### 3. Get Bus Owner Profile
```bash
curl -X GET http://localhost:8080/api/v1/bus-owner/profile \
  -H "Authorization: Bearer <token>"

# Expected: 200 OK, profile_completed=true
```

#### 4. Get Permits
```bash
curl -X GET http://localhost:8080/api/v1/route-permits \
  -H "Authorization: Bearer <token>"

# Expected: 200 OK, array with 1 permit
```

#### 5. Update Permit
```bash
curl -X PUT http://localhost:8080/api/v1/route-permits/<permit-id> \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "approved_fare": 120.00,
    "via": "City C, City D, City E"
  }'

# Expected: 200 OK
```

#### 6. Delete Permit
```bash
curl -X DELETE http://localhost:8080/api/v1/route-permits/<permit-id> \
  -H "Authorization: Bearer <token>"

# Expected: 200 OK
```

---

## 🎯 Integration with Flutter App

Once backend is ready, update Flutter app:

1. Update API base URL in `lib/config/api_config.dart`
2. Test onboarding flow:
   - Login with phone ending in "0000"
   - Complete onboarding form
   - Add permit
   - Verify navigation to home screen
3. Test bus adding:
   - Try adding bus with matching license plate
   - Try adding bus with non-matching license plate (should fail)
4. Test schedule creation:
   - Select permit
   - Verify bus auto-selects

---

## 📚 Code Examples for Reference

Check existing implementations:
- **Handler Pattern:** `internal/handlers/staff_handler.go`
- **Repository Pattern:** `internal/database/bus_staff_repository.go`
- **Model Validation:** `internal/models/bus_staff.go`
- **JWT Middleware:** `internal/middleware/auth_middleware.go`
- **Route Setup:** `cmd/server/main.go` (lines 150-250)

---

## 🚀 Deployment Notes

### Environment Variables
Add to `.env`:
```
# Already exists - no changes needed
DATABASE_URL=postgresql://...
JWT_SECRET=...
```

### Database Migration
```bash
# Development
psql -U postgres -d smart_transit_dev -f migrations/001_bus_owner_onboarding.sql

# Production
# 1. Backup database first
pg_dump -U postgres smart_transit > backup.sql

# 2. Apply migration
psql -U postgres -d smart_transit -f migrations/001_bus_owner_onboarding.sql

# 3. Verify
psql -U postgres -d smart_transit -c "\d route_permits"
```

---

## ✅ Summary

**Completed (50%):**
- ✅ Database schema & migration
- ✅ Data models (BusOwner, RoutePermit)
- ✅ Repository layer (RoutePermitRepository)

**Remaining (50%):**
- 🔨 Update BusOwnerRepository queries
- 🔨 Create BusOwnerHandler
- 🔨 Create PermitHandler
- 🔨 Wire routes in main.go
- 🔨 Test all endpoints
- 🔨 Deploy and integrate with Flutter app

**Estimated Time:** 3-4 hours to complete remaining work.
