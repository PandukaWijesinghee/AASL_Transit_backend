# Scheduled Trips Search Fix

## Problem
Scheduled trips were not appearing in search results even when valid locations were searched.

## Root Cause
**Type mismatch in SQL queries** - The search query was comparing UUID parameters with text arrays without proper type casting.

### Specific Issues Found:

1. **Line 236-237**: Comparing `fromStopID` and `toStopID` (UUID) with `selected_stop_ids` (text array)
   - Before: `$1 = ANY(bor.selected_stop_ids)`
   - After: `$1::text = ANY(bor.selected_stop_ids)`

2. **Line 474**: Similar issue when fetching route stops
   - Before: `mrs.id = ANY(bor.selected_stop_ids)`
   - After: `mrs.id::text = ANY(bor.selected_stop_ids)`

3. **Missing NULL checks**: Query didn't handle NULL or empty `selected_stop_ids` arrays
   - Added: `OR bor.selected_stop_ids IS NULL`
   - Added: `OR array_length(bor.selected_stop_ids, 1) IS NULL`

## Changes Made

### File: `backend/internal/database/search_repository.go`

#### Change 1: Fixed stop ID comparison in trip search (Lines 223-239)
```go
-- For bus owner routes, check if stops are selected (cast UUID to text for comparison)
AND (
    bor.id IS NULL
    OR bor.selected_stop_ids IS NULL
    OR array_length(bor.selected_stop_ids, 1) IS NULL
    OR (
        $1::text = ANY(bor.selected_stop_ids)
        AND $2::text = ANY(bor.selected_stop_ids)
    )
)
```

#### Change 2: Fixed stop fetching for route details (Line 474)
```go
AND mrs.id::text = ANY(bor.selected_stop_ids)
```

## Impact
✅ Scheduled trips will now appear in search results
✅ Bus owner routes with selected stops will work correctly
✅ Handles edge cases (NULL arrays, empty arrays)
✅ Proper type casting prevents PostgreSQL errors

## Deployment Steps

1. **Commit Changes**
   ```bash
   cd C:\Users\pandu\Documents\AASL\Workspace\passengerApp\AASL_Transit_backend
   git add backend/internal/database/search_repository.go
   git commit -m "Fix: Cast UUIDs to text when comparing with selected_stop_ids array"
   git push
   ```

2. **Deploy to Choreo**
   - Choreo will auto-deploy, OR
   - Manually trigger deployment in Choreo Console

3. **Test**
   - Search for trips between two locations
   - Verify scheduled trips appear in results
   - Check that bus owner routes work correctly

## Testing Checklist
- [ ] Search for trips between known stops
- [ ] Verify trips appear with correct details
- [ ] Test with both master routes and bus owner routes
- [ ] Check that stops are correctly filtered based on selected_stop_ids
- [ ] Verify empty results show appropriate message

## Additional Notes

### Database Schema Note
The `selected_stop_ids` column in `bus_owner_routes` table is defined as just `ARRAY` without specifying type. In practice, it stores text/varchar values representing UUIDs. This is why explicit casting is needed.

### Why This Matters
- PostgreSQL `ANY()` operator is type-strict
- UUID and text are different types
- Without casting, the comparison silently fails
- Result: No trips returned even when they exist

## Rollback
If issues occur, revert the commit:
```bash
git revert HEAD
git push
```
