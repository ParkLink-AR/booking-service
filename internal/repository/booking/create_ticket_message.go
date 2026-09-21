package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateTicketMessage inserts a message on a support ticket's thread and fills in its ID and
// CreatedAt. The ticket's opening message (its "description") is created this same way, with
// IsAdmin false, right after the ticket row itself -- see usecase.CreateTicketUseCase.
func (repository *Repository) CreateTicketMessage(ctx context.Context, message entity.SupportTicketMessage) (entity.SupportTicketMessage, error) {
	row := repository.database.QueryRowContext(ctx, `
		INSERT INTO `+constants.SupportTicketMessagesTableName+` (ticket_id, sender_id, is_admin, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, message.TicketID, message.SenderID, message.IsAdmin, message.Body)

	if err := row.Scan(&message.ID, &message.CreatedAt); err != nil {
		return entity.SupportTicketMessage{}, errors.NewInternalError("booking.CreateTicketMessage", err)
	}
	return message, nil
}
