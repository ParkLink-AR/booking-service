package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateMessage inserts a booking message and fills in its ID and CreatedAt.
func (repository *Repository) CreateMessage(ctx context.Context, message entity.BookingMessage) (entity.BookingMessage, error) {
	row := repository.database.QueryRowContext(ctx, `
		INSERT INTO `+constants.BookingMessagesTableName+` (booking_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, message.BookingID, message.SenderID, message.Body)

	if err := row.Scan(&message.ID, &message.CreatedAt); err != nil {
		return entity.BookingMessage{}, errors.NewInternalError("booking.CreateMessage", err)
	}
	return message, nil
}
