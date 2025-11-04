-- Migration: Add Schedule Improvements
-- Description: Add direction, trips_per_day, and estimated_arrival_time fields
-- Date: 2025-11-04

BEGIN;

-- =====================================================
-- 1. ADD MISSING FIELDS TO trip_schedules
-- =====================================================

-- Add direction field (UP/DOWN/ROUND_TRIP)
ALTER TABLE public.trip_schedules
ADD COLUMN IF NOT EXISTS direction VARCHAR NOT NULL DEFAULT 'UP'
CHECK (direction IN ('UP', 'DOWN', 'ROUND_TRIP'));

-- Add trips_per_day field
ALTER TABLE public.trip_schedules
ADD COLUMN IF NOT EXISTS trips_per_day INTEGER NOT NULL DEFAULT 1
CHECK (trips_per_day > 0 AND trips_per_day <= 10);

-- Add estimated_arrival_time field
ALTER TABLE public.trip_schedules
ADD COLUMN IF NOT EXISTS estimated_arrival_time TIME;

-- Add index for direction queries
CREATE INDEX IF NOT EXISTS idx_trip_schedules_direction
ON public.trip_schedules(direction);

-- =====================================================
-- 2. ADD COMMENTS FOR DOCUMENTATION
-- =====================================================

COMMENT ON COLUMN public.trip_schedules.direction IS 'Trip direction: UP (origin to destination), DOWN (destination to origin), or ROUND_TRIP (both)';
COMMENT ON COLUMN public.trip_schedules.trips_per_day IS 'Number of trips per day for this schedule (1-10)';
COMMENT ON COLUMN public.trip_schedules.estimated_arrival_time IS 'Manually entered estimated arrival time at final destination';

COMMIT;
