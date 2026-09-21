package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateTicket inserts a support ticket in OPEN status and fills in its ID, timestamps and status.
func (repository *Repository) CreateTicket(ctx context.Context, ticket entity.SupportTicket) (entity.SupportTicket, error) {
	row := repository.database.QueryRowContext(ctx, `
		INSERT INTO `+constants.SupportTicketsTableName+` (booking_id, created_by, subject)
		VALUES ($1, $2, $3)
		RETURNING id, status, created_at, updated_at
	`, ticket.BookingID, ticket.CreatedBy, ticket.Subject)

	var statusLabel string
	if err := row.Scan(&ticket.ID, &statusLabel, &ticket.CreatedAt, &ticket.UpdatedAt); err != nil {
		return entity.SupportTicket{}, errors.NewInternalError("booking.CreateTicket", err)
	}
	ticket.Status, _ = enum.ParseTicketStatus(statusLabel)
	return ticket, nil
}
