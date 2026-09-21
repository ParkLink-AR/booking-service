package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetTicketByID fetches a single support ticket by id, regardless of status.
func (repository *Repository) GetTicketByID(ctx context.Context, ticketID string) (entity.SupportTicket, error) {
	row := repository.database.QueryRowContext(ctx, `
		SELECT id, booking_id, created_by, subject, status, created_at, updated_at
		FROM `+constants.SupportTicketsTableName+`
		WHERE id = $1
	`, ticketID)

	var ticket entity.SupportTicket
	var statusLabel string
	if err := row.Scan(
		&ticket.ID, &ticket.BookingID, &ticket.CreatedBy, &ticket.Subject,
		&statusLabel, &ticket.CreatedAt, &ticket.UpdatedAt,
	); err != nil {
		return entity.SupportTicket{}, errors.NewNotFound("support_ticket", ticketID)
	}
	ticket.Status, _ = enum.ParseTicketStatus(statusLabel)
	return ticket, nil
}
