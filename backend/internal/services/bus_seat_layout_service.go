package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusSeatLayoutService handles bus seat layout business logic
type BusSeatLayoutService struct {
	repo *database.BusSeatLayoutRepository
}

// NewBusSeatLayoutService creates a new bus seat layout service
func NewBusSeatLayoutService(repo *database.BusSeatLayoutRepository) *BusSeatLayoutService {
	return &BusSeatLayoutService{
		repo: repo,
	}
}

// CreateTemplate creates a new bus seat layout template with intelligent seat generation
func (s *BusSeatLayoutService) CreateTemplate(ctx context.Context, req *models.CreateBusSeatLayoutTemplateRequest, adminID uuid.UUID) (*models.BusSeatLayoutTemplateResponse, error) {
	// Validate seat map
	if len(req.SeatMap) != req.TotalRows {
		return nil, fmt.Errorf("seat map rows (%d) must match total_rows (%d)", len(req.SeatMap), req.TotalRows)
	}

	// Generate seats from seat map
	seats := s.generateSeatsFromMap(req.SeatMap)

	// Create template
	template := &models.BusSeatLayoutTemplate{
		TemplateName: req.TemplateName,
		TotalRows:    req.TotalRows,
		TotalSeats:   len(seats),
		Description:  req.Description,
		IsActive:     true,
		CreatedBy:    adminID,
	}

	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	// Set template ID for all seats
	for i := range seats {
		seats[i].TemplateID = template.ID
	}

	// Create seats
	if err := s.repo.CreateSeats(ctx, seats); err != nil {
		return nil, fmt.Errorf("failed to create seats: %w", err)
	}

	// Build response
	return s.buildTemplateResponse(template, seats), nil
}

// generateSeatsFromMap generates seat objects from the boolean seat map
// seat_map: [row][position] where position 0-2 is left side, 3-5 is right side
func (s *BusSeatLayoutService) generateSeatsFromMap(seatMap [][]bool) []models.BusSeatLayoutSeat {
	var seats []models.BusSeatLayoutSeat

	for rowIdx, row := range seatMap {
		if len(row) != 6 {
			continue // Skip invalid rows
		}

		rowNumber := rowIdx + 1
		rowLabel := getRowLabel(rowNumber)

		// Find selected seats in this row
		var selectedPositions []int
		for pos, selected := range row {
			if selected {
				selectedPositions = append(selectedPositions, pos)
			}
		}

		// Generate seats for selected positions with continuous numbering
		totalSeatsInRow := len(selectedPositions)
		seatCounter := 0

		// Generate seats in order (left side first, then right side)
		for _, pos := range selectedPositions {
			seatCounter++
			isLeftSide := pos < 3

			seat := models.BusSeatLayoutSeat{
				RowNumber:    rowNumber,
				RowLabel:     rowLabel,
				Position:     pos + 1, // Convert 0-indexed to 1-indexed
				IsWindowSeat: seatCounter == 1 || seatCounter == totalSeatsInRow, // First or last seat
				IsAisleSeat:  (isLeftSide && pos == 2) || (!isLeftSide && pos == 3), // Aisle positions
			}
			seat.SeatNumber = s.generateSeatNumber(rowLabel, totalSeatsInRow, seatCounter)
			seats = append(seats, seat)
		}
	}

	return seats
}

// generateSeatNumber generates a seat number like A1W, A2, B3W
func (s *BusSeatLayoutService) generateSeatNumber(rowLabel string, totalSeatsInRow int, seatNumber int) string {
	// Check if it's a window seat (first or last seat in the row)
	isWindow := seatNumber == 1 || seatNumber == totalSeatsInRow

	if isWindow {
		return fmt.Sprintf("%s%dW", rowLabel, seatNumber)
	}
	return fmt.Sprintf("%s%d", rowLabel, seatNumber)
}

// getRowLabel converts row number to alphabetic label (1->A, 2->B, etc.)
func getRowLabel(rowNumber int) string {
	if rowNumber <= 0 {
		return "A"
	}
	if rowNumber <= 26 {
		return string(rune('A' + rowNumber - 1))
	}
	// For rows > 26, use AA, AB, etc.
	first := (rowNumber - 1) / 26
	second := (rowNumber - 1) % 26
	return string(rune('A'+first-1)) + string(rune('A'+second))
}

// GetTemplateByID retrieves a template with all seats and layout preview
func (s *BusSeatLayoutService) GetTemplateByID(ctx context.Context, templateID uuid.UUID) (*models.BusSeatLayoutTemplateResponse, error) {
	template, err := s.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return nil, err
	}

	seats, err := s.repo.GetSeatsByTemplateID(ctx, templateID)
	if err != nil {
		return nil, err
	}

	return s.buildTemplateResponse(template, seats), nil
}

// ListTemplates retrieves all templates
func (s *BusSeatLayoutService) ListTemplates(ctx context.Context, activeOnly bool) ([]*models.BusSeatLayoutTemplateResponse, error) {
	templates, err := s.repo.ListTemplates(ctx, activeOnly)
	if err != nil {
		return nil, err
	}

	var responses []*models.BusSeatLayoutTemplateResponse
	for _, template := range templates {
		seats, err := s.repo.GetSeatsByTemplateID(ctx, template.ID)
		if err != nil {
			continue // Skip templates with errors
		}
		responses = append(responses, s.buildTemplateResponse(template, seats))
	}

	return responses, nil
}

// UpdateTemplate updates a template's basic information
func (s *BusSeatLayoutService) UpdateTemplate(ctx context.Context, templateID uuid.UUID, req *models.UpdateBusSeatLayoutTemplateRequest) error {
	return s.repo.UpdateTemplate(ctx, templateID, req)
}

// DeleteTemplate deletes a template
func (s *BusSeatLayoutService) DeleteTemplate(ctx context.Context, templateID uuid.UUID) error {
	return s.repo.DeleteTemplate(ctx, templateID)
}

// buildTemplateResponse builds a complete template response with layout preview
func (s *BusSeatLayoutService) buildTemplateResponse(template *models.BusSeatLayoutTemplate, seats []models.BusSeatLayoutSeat) *models.BusSeatLayoutTemplateResponse {
	// Group seats by row
	rowMap := make(map[int][]models.BusSeatLayoutSeat)
	for _, seat := range seats {
		rowMap[seat.RowNumber] = append(rowMap[seat.RowNumber], seat)
	}

	// Build layout preview
	var rows []models.BusRow
	for rowNum := 1; rowNum <= template.TotalRows; rowNum++ {
		rowSeats := rowMap[rowNum]
		if len(rowSeats) == 0 {
			continue
		}

		row := models.BusRow{
			RowNumber:  rowNum,
			RowLabel:   rowSeats[0].RowLabel,
			LeftSeats:  []models.SeatInfo{},
			RightSeats: []models.SeatInfo{},
		}

		for _, seat := range rowSeats {
			seatInfo := models.SeatInfo{
				Position:     seat.Position,
				SeatNumber:   seat.SeatNumber,
				IsWindowSeat: seat.IsWindowSeat,
				IsAisleSeat:  seat.IsAisleSeat,
			}

			// Position 1-3 is left, 4-6 is right
			if seat.Position <= 3 {
				row.LeftSeats = append(row.LeftSeats, seatInfo)
			} else {
				row.RightSeats = append(row.RightSeats, seatInfo)
			}
		}

		rows = append(rows, row)
	}

	return &models.BusSeatLayoutTemplateResponse{
		ID:           template.ID,
		TemplateName: template.TemplateName,
		TotalRows:    template.TotalRows,
		TotalSeats:   template.TotalSeats,
		Description:  template.Description,
		IsActive:     template.IsActive,
		CreatedBy:    template.CreatedBy,
		CreatedAt:    template.CreatedAt,
		UpdatedAt:    template.UpdatedAt,
		Seats:        seats,
		LayoutPreview: models.BusLayoutPreview{
			Rows: rows,
		},
	}
}
