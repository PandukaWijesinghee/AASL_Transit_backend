package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DiagnosticTripSeats is a diagnostic endpoint to debug seat issues
// GET /api/v1/diagnostic/trip-seats/:id
func (h *TripSeatHandler) DiagnosticTripSeats(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	fmt.Printf("\n=== DIAGNOSTIC: TRIP SEATS FOR %s ===\n", tripID)

	// Try to fetch seats
	seats, err := h.tripSeatRepo.GetByScheduledTripIDWithBookingInfo(tripID)
	
	diagnostics := gin.H{
		"trip_id": tripID,
		"timestamp": "2026-03-06T15:00:00Z",
	}

	if err != nil {
		fmt.Printf("❌ Error fetching seats: %v\n", err)
		diagnostics["error"] = err.Error()
		diagnostics["seats_count"] = 0
		diagnostics["seats"] = []interface{}{}
	} else {
		fmt.Printf("✅ Successfully fetched %d seats\n", len(seats))
		diagnostics["seats_count"] = len(seats)
		
		// Convert seats to a more readable format
		var seatList []interface{}
		for i, seat := range seats {
			fmt.Printf("   [%d] %s (row:%d, col:%d, status:%s, price:%.2f)\n",
				i, seat.SeatNumber, seat.RowNumber, seat.Position, seat.Status, seat.SeatPrice)
			
			seatList = append(seatList, gin.H{
				"id":          seat.ID,
				"seat_number": seat.SeatNumber,
				"row":         seat.RowNumber,
				"column":      seat.Position,
				"status":      seat.Status,
				"price":       seat.SeatPrice,
				"type":        seat.SeatType,
			})
		}
		diagnostics["seats"] = seatList
	}

	// Also get summary
	summary, err := h.tripSeatRepo.GetSummary(tripID)
	if err != nil {
		fmt.Printf("⚠️ Error fetching summary: %v\n", err)
		diagnostics["summary_error"] = err.Error()
	} else if summary != nil {
		fmt.Printf("✅ Summary: total=%d, available=%d, booked=%d, blocked=%d\n",
			summary.TotalSeats, summary.AvailableSeats, summary.BookedSeats, summary.BlockedSeats)
		diagnostics["summary"] = gin.H{
			"total_seats":     summary.TotalSeats,
			"available_seats": summary.AvailableSeats,
			"booked_seats":    summary.BookedSeats,
			"blocked_seats":   summary.BlockedSeats,
			"reserved_seats":  summary.ReservedSeats,
		}
	}

	fmt.Printf("=== END DIAGNOSTIC ===\n\n")

	c.JSON(http.StatusOK, diagnostics)
}
