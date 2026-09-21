package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListTicketsByBookingAndCreator returns every ticket the caller opened on a booking, newest
// first. Each side of a booking only sees their own tickets, not the other side's.
func (repository *Repository) ListTicketsByBookingAndCreator(ctx context.Context, bookingID, createdBy string) ([]entity.SupportTicket, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, booking_id, created_by, subject, status, created_at, updated_at
		FROM `+constants.SupportTicketsTableName+`
		WHERE booking_id = $1 AND created_by = $2
		ORDER BY created_at DESC
	`, bookingID, createdBy)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListTicketsByBookingAndCreator", err)
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
			return nil, errors.NewInternalError("booking.ListTicketsByBookingAndCreator", err)
		}
		ticket.Status, _ = enum.ParseTicketStatus(statusLabel)
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListTicketsByBookingAndCreator", err)
	}
	return tickets, nil
}
