package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListMessagesByTicket returns every message on a support ticket's thread, oldest first.
func (repository *Repository) ListMessagesByTicket(ctx context.Context, ticketID string) ([]entity.SupportTicketMessage, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, ticket_id, sender_id, is_admin, body, created_at
		FROM `+constants.SupportTicketMessagesTableName+`
		WHERE ticket_id = $1
		ORDER BY created_at ASC
	`, ticketID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListMessagesByTicket", err)
	}
	defer rows.Close()

	var messages []entity.SupportTicketMessage
	for rows.Next() {
		var m entity.SupportTicketMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.SenderID, &m.IsAdmin, &m.Body, &m.CreatedAt); err != nil {
			return nil, errors.NewInternalError("booking.ListMessagesByTicket", err)
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListMessagesByTicket", err)
	}
	return messages, nil
}
