-- ============================================================================
-- Migration: Add Profile Completion and Make Fields Optional for Bus Staff
-- Date: October 14, 2025
-- Purpose: Support driver/conductor app registration flow
-- ============================================================================

-- STEP 1: Add 'pending' value to employment_status enum
-- This MUST be done outside a transaction block
ALTER TYPE employment_status ADD VALUE IF NOT EXISTS 'pending';

-- STEP 2: Apply all other changes in a transaction
BEGIN;

-- 2. Add profile_completed flag
ALTER TABLE public.bus_staff 
ADD COLUMN IF NOT EXISTS profile_completed BOOLEAN DEFAULT FALSE;

-- 3. Make bus_owner_id optional (can be assigned later by admin)
ALTER TABLE public.bus_staff 
ALTER COLUMN bus_owner_id DROP NOT NULL;

-- 4. Make hire_date optional (set when approved)
ALTER TABLE public.bus_staff 
ALTER COLUMN hire_date DROP NOT NULL;

-- 5. Update default employment status to pending
ALTER TABLE public.bus_staff 
ALTER COLUMN employment_status SET DEFAULT 'pending';

-- 6. Add verification fields
ALTER TABLE public.bus_staff 
ADD COLUMN IF NOT EXISTS verification_notes TEXT,
ADD COLUMN IF NOT EXISTS verified_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN IF NOT EXISTS verified_by UUID REFERENCES public.users(id);

-- 7. Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_bus_staff_profile_completed 
ON public.bus_staff(profile_completed);

CREATE INDEX IF NOT EXISTS idx_bus_staff_user_id 
ON public.bus_staff(user_id);

CREATE INDEX IF NOT EXISTS idx_bus_staff_bus_owner_id 
ON public.bus_staff(bus_owner_id) WHERE bus_owner_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_bus_staff_employment_status 
ON public.bus_staff(employment_status);

CREATE INDEX IF NOT EXISTS idx_bus_staff_staff_type 
ON public.bus_staff(staff_type);

-- 8. Add comments
COMMENT ON COLUMN public.bus_staff.profile_completed IS 
'Indicates if driver/conductor has completed their registration profile';

COMMENT ON COLUMN public.bus_staff.bus_owner_id IS 
'Optional during registration - can be assigned later by admin or bus owner';

COMMENT ON COLUMN public.bus_staff.verification_notes IS 
'Admin notes about background check or verification status';

COMMENT ON COLUMN public.bus_staff.verified_at IS 
'Timestamp when staff was verified by admin';

COMMENT ON COLUMN public.bus_staff.verified_by IS 
'Admin user ID who verified this staff member';

COMMIT;

-- Verification query to check changes
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns
WHERE table_name = 'bus_staff'
    AND table_schema = 'public'
ORDER BY ordinal_position;
