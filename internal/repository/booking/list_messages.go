package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListMessagesByBooking returns every message on a booking, oldest first (chat order).
func (repository *Repository) ListMessagesByBooking(ctx context.Context, bookingID string) ([]entity.BookingMessage, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, booking_id, sender_id, body, created_at
		FROM `+constants.BookingMessagesTableName+`
		WHERE booking_id = $1
		ORDER BY created_at ASC
	`, bookingID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListMessagesByBooking", err)
	}
	defer rows.Close()

	var messages []entity.BookingMessage
	for rows.Next() {
		var m entity.BookingMessage
		if err := rows.Scan(&m.ID, &m.BookingID, &m.SenderID, &m.Body, &m.CreatedAt); err != nil {
			return nil, errors.NewInternalError("booking.ListMessagesByBooking", err)
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListMessagesByBooking", err)
	}
	return messages, nil
}
