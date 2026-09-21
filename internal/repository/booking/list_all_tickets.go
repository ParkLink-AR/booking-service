package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListAllTickets returns every support ticket across every booking, newest first, for
// ParkLink staff triage. When status is non-nil, only tickets in that status are returned.
func (repository *Repository) ListAllTickets(ctx context.Context, status *enum.TicketStatus) ([]entity.SupportTicket, error) {
	query := `
		SELECT id, booking_id, created_by, subject, status, created_at, updated_at
		FROM ` + constants.SupportTicketsTableName + `
	`
	args := []any{}
	if status != nil {
		query += "WHERE status = $1\n"
		args = append(args, status.String())
	}
	query += "ORDER BY created_at DESC"

	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListAllTickets", err)
	}
	defer rows.Close()

	var tickets []entity.SupportTicket
	for rows.Next() {
		var ticket entity.SupportTicket
		var statusLabel string
		if err := rows.Scan(
			&ticket.ID, &ticket.BookingID, &ticket.CreatedBy, &ticket.Subject,
			&statusLabel, &ticket.CreatedAt, &ticket.UpdatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListAllTickets", err)
		}
		ticket.Status, _ = enum.ParseTicketStatus(statusLabel)
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListAllTickets", err)
	}
	return tickets, nil
}
