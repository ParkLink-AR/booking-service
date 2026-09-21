package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// UpdateTicketStatus sets a support ticket's status and bumps its updated_at, returning the
// row as it now stands.
func (repository *Repository) UpdateTicketStatus(ctx context.Context, ticketID string, status enum.TicketStatus) (entity.SupportTicket, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.SupportTicketsTableName+`
		SET status = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, booking_id, created_by, subject, status, created_at, updated_at
	`, ticketID, status.String())

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
