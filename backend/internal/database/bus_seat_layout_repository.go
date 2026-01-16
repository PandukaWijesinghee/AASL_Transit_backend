package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusSeatLayoutRepository handles bus seat layout database operations
type BusSeatLayoutRepository struct {
	db DB
}

// NewBusSeatLayoutRepository creates a new bus seat layout repository
func NewBusSeatLayoutRepository(db DB) *BusSeatLayoutRepository {
	return &BusSeatLayoutRepository{
		db: db,
	}
}

// CreateTemplate creates a new bus seat layout template
func (r *BusSeatLayoutRepository) CreateTemplate(ctx context.Context, template *models.BusSeatLayoutTemplate) error {
	query := `
		INSERT INTO bus_seat_layout_templates (
			template_name, total_rows, total_seats, description,
			is_active, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		template.TemplateName,
		template.TotalRows,
		template.TotalSeats,
		template.Description,
		template.IsActive,
		template.CreatedBy,
	).Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create bus seat layout template: %w", err)
	}

	return nil
}

// CreateSeats creates multiple seats for a template
func (r *BusSeatLayoutRepository) CreateSeats(ctx context.Context, seats []models.BusSeatLayoutSeat) error {
	if len(seats) == 0 {
		return nil
	}

	query := `
		INSERT INTO bus_seat_layout_seats (
			template_id, row_number, row_label, position,
			seat_number, is_window_seat, is_aisle_seat, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`

	for _, seat := range seats {
		_, err := r.db.Exec(
			query,
			seat.TemplateID,
			seat.RowNumber,
			seat.RowLabel,
			seat.Position,
			seat.SeatNumber,
			seat.IsWindowSeat,
			seat.IsAisleSeat,
		)
		if err != nil {
			return fmt.Errorf("failed to insert seat %s: %w", seat.SeatNumber, err)
		}
	}

	return nil
}

// GetTemplateByID retrieves a template by ID
func (r *BusSeatLayoutRepository) GetTemplateByID(ctx context.Context, templateID uuid.UUID) (*models.BusSeatLayoutTemplate, error) {
	var template models.BusSeatLayoutTemplate

	query := `
		SELECT id, template_name, total_rows, total_seats, description,
		       is_active, created_by, created_at, updated_at
		FROM bus_seat_layout_templates
		WHERE id = $1
	`

	err := r.db.Get(&template, query, templateID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template not found")
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return &template, nil
}

// GetSeatsByTemplateID retrieves all seats for a template
func (r *BusSeatLayoutRepository) GetSeatsByTemplateID(ctx context.Context, templateID uuid.UUID) ([]models.BusSeatLayoutSeat, error) {
	var seats []models.BusSeatLayoutSeat

	query := `
		SELECT id, template_id, row_number, row_label, position,
		       seat_number, is_window_seat, is_aisle_seat, created_at
		FROM bus_seat_layout_seats
		WHERE template_id = $1
		ORDER BY row_number, position
	`

	err := r.db.Select(&seats, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seats: %w", err)
	}

	return seats, nil
}

// ListTemplates retrieves all templates with optional filters
func (r *BusSeatLayoutRepository) ListTemplates(ctx context.Context, activeOnly bool) ([]*models.BusSeatLayoutTemplate, error) {
	var templates []*models.BusSeatLayoutTemplate

	query := `
		SELECT id, template_name, total_rows, total_seats, description,
		       is_active, created_by, created_at, updated_at
		FROM bus_seat_layout_templates
	`

	if activeOnly {
		query += " WHERE is_active = true"
	}

	query += " ORDER BY created_at DESC"

	err := r.db.Select(&templates, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	return templates, nil
}

// UpdateTemplate updates a template's basic information
func (r *BusSeatLayoutRepository) UpdateTemplate(ctx context.Context, templateID uuid.UUID, req *models.UpdateBusSeatLayoutTemplateRequest) error {
	query := `
		UPDATE bus_seat_layout_templates
		SET
			template_name = COALESCE($1, template_name),
			description = COALESCE($2, description),
			is_active = COALESCE($3, is_active),
			updated_at = NOW()
		WHERE id = $4
	`

	result, err := r.db.Exec(query, req.TemplateName, req.Description, req.IsActive, templateID)
	if err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("template not found")
	}

	return nil
}

// DeleteTemplate deletes a template (cascades to seats)
func (r *BusSeatLayoutRepository) DeleteTemplate(ctx context.Context, templateID uuid.UUID) error {
	query := `DELETE FROM bus_seat_layout_templates WHERE id = $1`

	result, err := r.db.Exec(query, templateID)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("template not found")
	}

	return nil
}

// UpdateTotalSeats updates the total seats count for a template
func (r *BusSeatLayoutRepository) UpdateTotalSeats(ctx context.Context, templateID uuid.UUID, totalSeats int) error {
	query := `
		UPDATE bus_seat_layout_templates
		SET total_seats = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(query, totalSeats, templateID)
	if err != nil {
		return fmt.Errorf("failed to update total seats: %w", err)
	}

	return nil
}
