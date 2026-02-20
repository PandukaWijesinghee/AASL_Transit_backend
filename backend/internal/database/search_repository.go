package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// SearchRepository handles database operations for search functionality
type SearchRepository struct {
	db *sqlx.DB
}

// NewSearchRepository creates a new search repository
func NewSearchRepository(db DB) *SearchRepository {
	// Type assertion to get *sqlx.DB
	if postgresDB, ok := db.(*PostgresDB); ok {
		return &SearchRepository{db: postgresDB.DB}
	}
	// Fallback - this should not happen in practice
	panic("Invalid database type for SearchRepository")
}

// FindStopByName finds a stop by exact name match (case-insensitive)
func (r *SearchRepository) FindStopByName(stopName string) (*models.StopInfo, *uuid.UUID, error) {
	query := `
		SELECT
			s.id,
			s.stop_name,
			COUNT(DISTINCT s.master_route_id) as route_count
		FROM master_route_stops s
		JOIN master_routes r ON s.master_route_id = r.id
		WHERE LOWER(s.stop_name) = LOWER($1)
		  AND r.is_active = true
		GROUP BY s.id, s.stop_name
		ORDER BY route_count DESC
		LIMIT 1
	`

	var result struct {
		ID         uuid.UUID `db:"id"`
		StopName   string    `db:"stop_name"`
		RouteCount int       `db:"route_count"`
	}

	err := r.db.Get(&result, query, strings.TrimSpace(stopName))
	if err != nil {
		if err == sql.ErrNoRows {
			// Stop not found - not an error, just return nil
			return &models.StopInfo{
				Matched:       false,
				OriginalInput: stopName,
			}, nil, nil
		}
		return nil, nil, fmt.Errorf("error finding stop: %w", err)
	}

	stopInfo := &models.StopInfo{
		ID:            &result.ID,
		Name:          result.StopName,
		Matched:       true,
		OriginalInput: stopName,
	}

	return stopInfo, &result.ID, nil
}

// StopPairResult holds the result of finding two stops on the same route
type StopPairResult struct {
	FromStop  *models.StopInfo
	ToStop    *models.StopInfo
	FromID    uuid.UUID
	ToID      uuid.UUID
	RouteID   uuid.UUID
	RouteName string
	Matched   bool
}

// FindStopPairOnSameRoute finds two stops that are on the same route with fuzzy matching
// This ensures both stops can be connected by a trip
func (r *SearchRepository) FindStopPairOnSameRoute(fromName, toName string) (*StopPairResult, error) {
	query := `
		SELECT
			from_stop.id as from_id,
			from_stop.stop_name as from_name,
			to_stop.id as to_id,
			to_stop.stop_name as to_name,
			from_stop.master_route_id as route_id,
			mr.route_name,
			from_stop.stop_order as from_order,
			to_stop.stop_order as to_order
		FROM master_route_stops from_stop
		INNER JOIN master_route_stops to_stop
			ON from_stop.master_route_id = to_stop.master_route_id
		INNER JOIN master_routes mr ON mr.id = from_stop.master_route_id
		WHERE
			LOWER(from_stop.stop_name) LIKE LOWER('%' || $1 || '%')
			AND LOWER(to_stop.stop_name) LIKE LOWER('%' || $2 || '%')
			AND from_stop.stop_order < to_stop.stop_order
			AND mr.is_active = true
		ORDER BY
			-- Exact stop name match first
			CASE WHEN LOWER(from_stop.stop_name) = LOWER($1) THEN 0 ELSE 1 END,
			CASE WHEN LOWER(to_stop.stop_name) = LOWER($2) THEN 0 ELSE 1 END,
			-- Prefer routes whose name or destination city contains the destination term.
			-- This ensures "Colombo - Kandy" beats "Colombo - Batticaloa" when searching for Kandy,
			-- because both routes share a "Kandy" stop but only the first is a Kandy route.
			CASE WHEN LOWER(mr.route_name) LIKE LOWER('%' || $2 || '%')
			          OR LOWER(mr.destination_city) LIKE LOWER('%' || $2 || '%')
			     THEN 0 ELSE 1 END,
			-- Also prefer routes whose origin matches from-term
			CASE WHEN LOWER(mr.route_name) LIKE LOWER('%' || $1 || '%')
			          OR LOWER(mr.origin_city) LIKE LOWER('%' || $1 || '%')
			     THEN 0 ELSE 1 END,
			-- Fall back to route with most stops (highest coverage)
			(SELECT COUNT(*) FROM master_route_stops WHERE master_route_id = mr.id) DESC
		LIMIT 1
	`

	var result struct {
		FromID    uuid.UUID `db:"from_id"`
		FromName  string    `db:"from_name"`
		ToID      uuid.UUID `db:"to_id"`
		ToName    string    `db:"to_name"`
		RouteID   uuid.UUID `db:"route_id"`
		RouteName string    `db:"route_name"`
		FromOrder int       `db:"from_order"`
		ToOrder   int       `db:"to_order"`
	}

	err := r.db.Get(&result, query, strings.TrimSpace(fromName), strings.TrimSpace(toName))
	if err != nil {
		if err == sql.ErrNoRows {
			// No stop pair found on same route
			return &StopPairResult{
				FromStop: &models.StopInfo{
					Matched:       false,
					OriginalInput: fromName,
				},
				ToStop: &models.StopInfo{
					Matched:       false,
					OriginalInput: toName,
				},
				Matched: false,
			}, nil
		}
		return nil, fmt.Errorf("error finding stop pair: %w", err)
	}

	// Successfully found both stops on the same route
	return &StopPairResult{
		FromStop: &models.StopInfo{
			ID:            &result.FromID,
			Name:          result.FromName,
			Matched:       true,
			OriginalInput: fromName,
		},
		ToStop: &models.StopInfo{
			ID:            &result.ToID,
			Name:          result.ToName,
			Matched:       true,
			OriginalInput: toName,
		},
		FromID:    result.FromID,
		ToID:      result.ToID,
		RouteID:   result.RouteID,
		RouteName: result.RouteName,
		Matched:   true,
	}, nil
}

// FindDirectTrips finds all direct trips between two stops
func (r *SearchRepository) FindDirectTrips(
	fromStopID, toStopID uuid.UUID,
	afterTime time.Time,
	limit int,
) ([]models.TripResult, error) {
	// Log search parameters
	fmt.Printf("\n🔍 === SEARCH QUERY DEBUG ===\n")
	fmt.Printf("FROM Stop ID: %s\n", fromStopID.String())
	fmt.Printf("TO Stop ID: %s\n", toStopID.String())
	fmt.Printf("After Time: %s\n", afterTime.Format(time.RFC3339))
	fmt.Printf("Limit: %d\n", limit)

	// Check how many scheduled trips exist that match basic criteria
	var debugCounts struct {
		TotalTrips    int `db:"total_trips"`
		BookableTrips int `db:"bookable_trips"`
		FutureTrips   int `db:"future_trips"`
		ValidStatus   int `db:"valid_status"`
		WithBORRoute  int `db:"with_bor_route"`
	}
	debugQuery := `
		SELECT 
			COUNT(*) as total_trips,
			COUNT(*) FILTER (WHERE is_bookable = true) as bookable_trips,
			COUNT(*) FILTER (WHERE departure_datetime > $1) as future_trips,
			COUNT(*) FILTER (WHERE status IN ('scheduled', 'confirmed')) as valid_status,
			COUNT(*) FILTER (WHERE bus_owner_route_id IS NOT NULL) as with_bor_route
		FROM scheduled_trips
	`
	if err := r.db.Get(&debugCounts, debugQuery, afterTime); err == nil {
		fmt.Printf("📊 Scheduled Trips Stats:\n")
		fmt.Printf("   Total: %d | Bookable: %d | Future: %d | Valid Status: %d | With Custom Route: %d\n",
			debugCounts.TotalTrips, debugCounts.BookableTrips, debugCounts.FutureTrips,
			debugCounts.ValidStatus, debugCounts.WithBORRoute)
	}

	query := `
		SELECT DISTINCT ON (st.id)
			st.id as trip_id,
			COALESCE(bor.custom_route_name, mr_bor.route_name, mr_permit.route_name) as route_name,
			COALESCE(mr_bor.route_number, mr_permit.route_number) as route_number,
			b.bus_type,
			st.departure_datetime as departure_time,
			-- Calculate arrival time: departure + duration
			st.departure_datetime +
				(COALESCE(st.estimated_duration_minutes, 0) * interval '1 minute') as estimated_arrival,
			COALESCE(st.estimated_duration_minutes, 0) as duration_minutes,
			-- Available seats removed - will be calculated from separate booking table
			COALESCE(bslt.total_seats, 0) as total_seats,
			COALESCE(rp.approved_fare, st.base_fare, 0) as fare,
			from_stop.stop_name as boarding_point,
			to_stop.stop_name as dropping_point,
			COALESCE(b.has_wifi, false) as has_wifi,
			COALESCE(b.has_ac, false) as has_ac,
			COALESCE(b.has_charging_ports, false) as has_charging_ports,
			COALESCE(b.has_entertainment, false) as has_entertainment,
			COALESCE(b.has_refreshments, false) as has_refreshments,
			st.is_bookable,
			-- Route info for fetching stops
			bor.id as bus_owner_route_id,
			-- Use bor.master_route_id first, fall back to permit's master_route_id
			COALESCE(bor.master_route_id, rp.master_route_id)::text as master_route_id
		FROM scheduled_trips st
		-- Join bus owner route
		LEFT JOIN bus_owner_routes bor ON st.bus_owner_route_id = bor.id
		-- Route name/number via bus_owner_route path
		LEFT JOIN master_routes mr_bor ON bor.master_route_id = mr_bor.id
		-- Join permit for fare and bus
		LEFT JOIN route_permits rp ON st.permit_id = rp.id
		-- Route name/number fallback via permit path (used when bus_owner_route_id is NULL)
		LEFT JOIN master_routes mr_permit ON rp.master_route_id = mr_permit.id
		LEFT JOIN buses b ON rp.bus_registration_number = b.license_plate
		-- Join seat layout template to get total_seats
		LEFT JOIN bus_seat_layout_templates bslt ON b.seat_layout_id = bslt.id
		-- Get stop information
		JOIN master_route_stops from_stop ON from_stop.id = $1
		JOIN master_route_stops to_stop ON to_stop.id = $2
		-- Verify stops are on the same route as the trip
		-- COALESCE: prefer bus_owner_route's master_route_id, fall back to permit's master_route_id
		JOIN master_route_stops check_from ON
			check_from.master_route_id = COALESCE(bor.master_route_id, rp.master_route_id)
			AND check_from.id = $1
		JOIN master_route_stops check_to ON
			check_to.master_route_id = COALESCE(bor.master_route_id, rp.master_route_id)
			AND check_to.id = $2
		WHERE
			-- Trip must be bookable and in valid status
			st.is_bookable = true
			AND st.status IN ('scheduled', 'confirmed')
			-- Departure must be in the future
			AND st.departure_datetime > $3
			-- Stops must be in correct order
			AND check_from.stop_order < check_to.stop_order
			-- For bus owner routes, check if stops are selected
			-- If selected_stop_ids is NULL or empty, treat as "all stops available"
			AND (
				bor.id IS NULL
				OR bor.selected_stop_ids IS NULL
				OR array_length(bor.selected_stop_ids, 1) IS NULL
				OR array_length(bor.selected_stop_ids, 1) = 0
				OR (
					$1 = ANY(bor.selected_stop_ids)
					AND $2 = ANY(bor.selected_stop_ids)
				)
			)
		ORDER BY st.id, st.departure_datetime
		LIMIT $4
	`

	// Use intermediate struct to scan flat SQL results
	type tripWithFeatures struct {
		TripID           uuid.UUID `db:"trip_id"`
		RouteName        string    `db:"route_name"`
		RouteNumber      *string   `db:"route_number"`
		BusType          *string   `db:"bus_type"` // Nullable - bus might not have type set
		DepartureTime    time.Time `db:"departure_time"`
		EstimatedArrival time.Time `db:"estimated_arrival"`
		DurationMinutes  int       `db:"duration_minutes"`
		TotalSeats       int       `db:"total_seats"`
		Fare             float64   `db:"fare"`
		BoardingPoint    string    `db:"boarding_point"`
		DroppingPoint    string    `db:"dropping_point"`
		HasWiFi          bool      `db:"has_wifi"`
		HasAC            bool      `db:"has_ac"`
		HasChargingPorts bool      `db:"has_charging_ports"`
		HasEntertainment bool      `db:"has_entertainment"`
		HasRefreshments  bool      `db:"has_refreshments"`
		IsBookable       bool      `db:"is_bookable"`
		// Route info for fetching stops
		BusOwnerRouteID *string `db:"bus_owner_route_id"`
		MasterRouteID   *string `db:"master_route_id"`
	}

	var tempTrips []tripWithFeatures
	err := r.db.Select(&tempTrips, query, fromStopID, toStopID, afterTime, limit)
	if err != nil {
		fmt.Printf("❌ SQL Query Error: %v\n", err)
		return nil, fmt.Errorf("error finding trips: %w", err)
	}

	fmt.Printf("✅ SQL Query successful - Found %d trips\n", len(tempTrips))

	// If no trips found, run diagnostic query to see why
	if len(tempTrips) == 0 {
		fmt.Printf("\n⚠️  NO TRIPS FOUND - Running diagnostics...\n")

		type diagnostic struct {
			TripID             uuid.UUID `db:"trip_id"`
			Departure          time.Time `db:"departure"`
			IsBookable         bool      `db:"is_bookable"`
			Status             string    `db:"status"`
			IsFuture           bool      `db:"is_future"`
			HasBORRoute        bool      `db:"has_bor_route"`
			SelectedStopsCount *int      `db:"selected_stops_count"`
			FromInSelected     *bool     `db:"from_in_selected"`
			ToInSelected       *bool     `db:"to_in_selected"`
			StopsConnected     bool      `db:"stops_connected"`
		}

		diagQuery := `
			SELECT 
				st.id as trip_id,
				st.departure_datetime as departure,
				st.is_bookable,
				st.status,
				st.departure_datetime > $3 as is_future,
				st.bus_owner_route_id IS NOT NULL as has_bor_route,
				CASE WHEN bor.id IS NOT NULL THEN array_length(bor.selected_stop_ids, 1) END as selected_stops_count,
				CASE WHEN bor.id IS NOT NULL AND bor.selected_stop_ids IS NOT NULL 
					THEN $1 = ANY(bor.selected_stop_ids) END as from_in_selected,
				CASE WHEN bor.id IS NOT NULL AND bor.selected_stop_ids IS NOT NULL 
					THEN $2 = ANY(bor.selected_stop_ids) END as to_in_selected,
				EXISTS (
					SELECT 1 
					FROM master_route_stops check_from
					JOIN master_route_stops check_to 
						ON check_from.master_route_id = check_to.master_route_id
					LEFT JOIN route_permits rp2 ON rp2.id = st.permit_id
					WHERE check_from.id = $1 
						AND check_to.id = $2
						AND check_from.stop_order < check_to.stop_order
						AND check_from.master_route_id = COALESCE(bor.master_route_id, rp2.master_route_id)
				) as stops_connected
			FROM scheduled_trips st
			LEFT JOIN bus_owner_routes bor ON st.bus_owner_route_id = bor.id
			WHERE st.departure_datetime > $3 - INTERVAL '24 hours'
			ORDER BY st.departure_datetime
			LIMIT 10
		`

		var diags []diagnostic
		if err := r.db.Select(&diags, diagQuery, fromStopID, toStopID, afterTime); err == nil {
			for _, d := range diags {
				reasons := []string{}
				if !d.IsBookable {
					reasons = append(reasons, "❌ NOT BOOKABLE")
				}
				if d.Status != "scheduled" && d.Status != "confirmed" {
					reasons = append(reasons, fmt.Sprintf("❌ WRONG STATUS: %s", d.Status))
				}
				if !d.IsFuture {
					reasons = append(reasons, "❌ PAST DEPARTURE")
				}
				// Check selected_stop_ids for custom routes
				if d.HasBORRoute {
					if d.SelectedStopsCount == nil || *d.SelectedStopsCount == 0 {
						reasons = append(reasons, "⚠️  selected_stop_ids is NULL/empty (now allowed)")
					} else if d.FromInSelected != nil && !*d.FromInSelected {
						reasons = append(reasons, "❌ FROM STOP NOT IN selected_stop_ids")
					} else if d.ToInSelected != nil && !*d.ToInSelected {
						reasons = append(reasons, "❌ TO STOP NOT IN selected_stop_ids")
					}
				}
				if !d.StopsConnected {
					reasons = append(reasons, "❌ STOPS NOT ON SAME ROUTE OR WRONG ORDER")
				}

				if len(reasons) == 0 {
					reasons = append(reasons, "✅ Should have matched!")
				}

				fmt.Printf("   Trip %s: %s | %s\n",
					d.TripID.String()[:8],
					d.Departure.Format("2006-01-02 15:04"),
					strings.Join(reasons, ", "))
			}
		}
		fmt.Printf("\n")
	}

	// Log each trip found for debugging
	for i, trip := range tempTrips {
		fmt.Printf("  Trip %d: %s (%s) - Departs: %s\n",
			i+1,
			trip.RouteName,
			trip.TripID.String()[:8],
			trip.DepartureTime.Format("2006-01-02 15:04"))
	}

	// Map to TripResult with nested BusFeatures
	trips := make([]models.TripResult, len(tempTrips))
	for i, temp := range tempTrips {
		// Handle nullable BusType - default to "Standard" if NULL
		busType := "Standard"
		if temp.BusType != nil {
			busType = *temp.BusType
		}

		trips[i] = models.TripResult{
			TripID:           temp.TripID,
			RouteName:        temp.RouteName,
			RouteNumber:      temp.RouteNumber,
			BusType:          busType,
			DepartureTime:    temp.DepartureTime,
			EstimatedArrival: temp.EstimatedArrival,
			DurationMinutes:  temp.DurationMinutes,
			TotalSeats:       temp.TotalSeats,
			Fare:             temp.Fare,
			BoardingPoint:    temp.BoardingPoint,
			DroppingPoint:    temp.DroppingPoint,
			BusFeatures: models.BusFeatures{
				HasWiFi:          temp.HasWiFi,
				HasAC:            temp.HasAC,
				HasChargingPorts: temp.HasChargingPorts,
				HasEntertainment: temp.HasEntertainment,
				HasRefreshments:  temp.HasRefreshments,
			},
			IsBookable:      temp.IsBookable,
			BusOwnerRouteID: temp.BusOwnerRouteID,
			MasterRouteID:   temp.MasterRouteID,
		}
	}

	return trips, nil
}

// LogSearch records a search query for analytics
func (r *SearchRepository) LogSearch(log *models.SearchLog) error {
	query := `
		INSERT INTO search_logs (
			from_input,
			to_input,
			from_stop_id,
			to_stop_id,
			results_count,
			response_time_ms,
			user_id,
			ip_address
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(
		query,
		log.FromInput,
		log.ToInput,
		log.FromStopID,
		log.ToStopID,
		log.ResultsCount,
		log.ResponseTimeMs,
		log.UserID,
		log.IPAddress,
	)

	if err != nil {
		return fmt.Errorf("error logging search: %w", err)
	}

	return nil
}

// GetPopularRoutes returns frequently searched routes
func (r *SearchRepository) GetPopularRoutes(limit int) ([]models.PopularRoute, error) {
	query := `
		SELECT
			from_input as from_stop_name,
			to_input as to_stop_name,
			COUNT(*) as search_count
		FROM search_logs
		WHERE from_stop_id IS NOT NULL
		  AND to_stop_id IS NOT NULL
		  AND created_at > NOW() - INTERVAL '30 days'
		GROUP BY from_input, to_input
		ORDER BY search_count DESC
		LIMIT $1
	`

	var routes []models.PopularRoute
	err := r.db.Select(&routes, query, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting popular routes: %w", err)
	}

	return routes, nil
}

// GetStopAutocomplete returns stop suggestions for autocomplete
func (r *SearchRepository) GetStopAutocomplete(searchTerm string, limit int) ([]models.StopAutocomplete, error) {
	query := `
		SELECT DISTINCT
			s.id as stop_id,
			s.stop_name,
			COUNT(DISTINCT s.master_route_id) as route_count
		FROM master_route_stops s
		JOIN master_routes r ON s.master_route_id = r.id
		WHERE LOWER(s.stop_name) LIKE LOWER($1)
		  AND r.is_active = true
		GROUP BY s.id, s.stop_name
		ORDER BY route_count DESC, s.stop_name
		LIMIT $2
	`

	searchPattern := "%" + strings.TrimSpace(searchTerm) + "%"

	var suggestions []models.StopAutocomplete
	err := r.db.Select(&suggestions, query, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting autocomplete suggestions: %w", err)
	}

	return suggestions, nil
}

// GetSearchAnalytics returns search analytics for admin dashboard
func (r *SearchRepository) GetSearchAnalytics(days int) (map[string]interface{}, error) {
	analytics := make(map[string]interface{})

	// Total searches
	var totalSearches int
	err := r.db.Get(&totalSearches, `
		SELECT COUNT(*)
		FROM search_logs
		WHERE created_at > NOW() - $1::INTERVAL
	`, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, err
	}
	analytics["total_searches"] = totalSearches

	// Average response time
	var avgResponseTime float64
	err = r.db.Get(&avgResponseTime, `
		SELECT COALESCE(AVG(response_time_ms), 0)
		FROM search_logs
		WHERE created_at > NOW() - $1::INTERVAL
	`, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, err
	}
	analytics["avg_response_time_ms"] = avgResponseTime

	// Success rate (searches with results)
	var successRate float64
	err = r.db.Get(&successRate, `
		SELECT COALESCE(
			100.0 * COUNT(CASE WHEN results_count > 0 THEN 1 END) / NULLIF(COUNT(*), 0),
			0
		)
		FROM search_logs
		WHERE created_at > NOW() - $1::INTERVAL
	`, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, err
	}
	analytics["success_rate"] = successRate

	return analytics, nil
}

// GetRouteStopsForTrip fetches the route stops for a trip based on bus_owner_route_id
// Returns stops ordered by stop_order for passenger to select boarding/alighting points
func (r *SearchRepository) GetRouteStopsForTrip(masterRouteID string, busOwnerRouteID *string) ([]models.RouteStop, error) {
	var stops []models.RouteStop

	if busOwnerRouteID != nil && *busOwnerRouteID != "" {
		// Bus owner has selected specific stops - only return those
		query := `
			SELECT 
				mrs.id,
				mrs.stop_name,
				mrs.stop_order,
				mrs.latitude,
				mrs.longitude,
				mrs.arrival_time_offset_minutes,
				mrs.is_major_stop
			FROM master_route_stops mrs
			JOIN bus_owner_routes bor ON bor.id = $1
			WHERE mrs.master_route_id = $2
			  AND mrs.id = ANY(bor.selected_stop_ids)
			ORDER BY mrs.stop_order ASC
		`
		err := r.db.Select(&stops, query, *busOwnerRouteID, masterRouteID)
		if err != nil {
			return nil, fmt.Errorf("error fetching route stops with bus owner route: %w", err)
		}
	} else {
		// No bus owner route - return all stops on master route
		query := `
			SELECT 
				id,
				stop_name,
				stop_order,
				latitude,
				longitude,
				arrival_time_offset_minutes,
				is_major_stop
			FROM master_route_stops
			WHERE master_route_id = $1
			ORDER BY stop_order ASC
		`
		err := r.db.Select(&stops, query, masterRouteID)
		if err != nil {
			return nil, fmt.Errorf("error fetching route stops: %w", err)
		}
	}

	return stops, nil
}
