package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListTicketsByUser returns every support ticket userID has ever opened, across every
// booking, newest-updated first, each enriched with its most recent staff reply (if any) via
// a LATERAL join -- one query instead of fanning out per ticket.
func (repository *Repository) ListTicketsByUser(ctx context.Context, userID string) ([]entity.TicketSummary, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT
			t.id, t.booking_id, t.created_by, t.subject, t.status, t.created_at, t.updated_at,
			lar.id, lar.created_at
		FROM `+constants.SupportTicketsTableName+` t
		LEFT JOIN LATERAL (
			SELECT m.id, m.created_at
			FROM `+constants.SupportTicketMessagesTableName+` m
			WHERE m.ticket_id = t.id AND m.is_admin = true
			ORDER BY m.created_at DESC
			LIMIT 1
		) lar ON true
		WHERE t.created_by = $1
		ORDER BY t.updated_at DESC
	`, userID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListTicketsByUser", err)
	}
	defer rows.Close()

	var summaries []entity.TicketSummary
	for rows.Next() {
		var s entity.TicketSummary
		var statusLabel string
		if err := rows.Scan(
			&s.ID, &s.BookingID, &s.CreatedBy, &s.Subject, &statusLabel, &s.CreatedAt, &s.UpdatedAt,
			&s.LastAdminReplyID, &s.LastAdminReplyAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListTicketsByUser", err)
		}
		s.Status, _ = enum.ParseTicketStatus(statusLabel)
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListTicketsByUser", err)
	}
	return summaries, nil
}
