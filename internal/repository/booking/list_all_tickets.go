package booking

import (
	"context"
	"database/sql"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListAllTickets returns every support ticket across every booking, newest first, for
// ParkLink staff triage -- each with its thread's last message (when, and whether staff sent
// it) and the booking it's about. When status is non-nil, only tickets in that status are
// returned.
func (repository *Repository) ListAllTickets(ctx context.Context, status *enum.TicketStatus) ([]entity.AdminTicket, error) {
	query := `
		SELECT t.id, t.booking_id, t.created_by, t.subject, t.status, t.created_at, t.updated_at,
		       last_message.created_at, COALESCE(last_message.is_admin, false),
		       b.user_id, b.parking_spot_id, lower(b.time_range), upper(b.time_range), b.status, b.kind
		FROM ` + constants.SupportTicketsTableName + ` t
		JOIN ` + constants.BookingsTableName + ` b ON b.id = t.booking_id
		LEFT JOIN LATERAL (
			SELECT m.created_at, m.is_admin
			FROM ` + constants.SupportTicketMessagesTableName + ` m
			WHERE m.ticket_id = t.id
			ORDER BY m.created_at DESC
			LIMIT 1
		) last_message ON true
	`
	args := []any{}
	if status != nil {
		query += "WHERE t.status = $1\n"
		args = append(args, status.String())
	}
	query += "ORDER BY t.created_at DESC"

	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListAllTickets", err)
	}
	defer rows.Close()

	var tickets []entity.AdminTicket
	for rows.Next() {
		var ticket entity.AdminTicket
		var statusLabel string
		var lastMessageAt sql.NullTime
		if err := rows.Scan(
			&ticket.ID, &ticket.BookingID, &ticket.CreatedBy, &ticket.Subject,
			&statusLabel, &ticket.CreatedAt, &ticket.UpdatedAt,
			&lastMessageAt, &ticket.LastMessageFromStaff,
			&ticket.BookingUserID, &ticket.ParkingSpotID, &ticket.BookingStart, &ticket.BookingEnd,
			&ticket.BookingStatus, &ticket.BookingKind,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListAllTickets", err)
		}
		ticket.Status, _ = enum.ParseTicketStatus(statusLabel)
		if lastMessageAt.Valid {
			ticket.LastMessageAt = &lastMessageAt.Time
		}
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListAllTickets", err)
	}
	return tickets, nil
}
