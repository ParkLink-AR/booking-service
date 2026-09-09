package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListBookingsByUser returns all bookings for a given user_id, ordered by start time descending.
func (repository *Repository) ListBookingsByUser(
	ctx context.Context,
	userID string,
) ([]entity.Booking, error) {
	query := `
		SELECT id, parking_spot_id, user_id,
		       lower(time_range), upper(time_range),
		       status, hold_expires_at, amount, COALESCE(payment_id, ''), kind, created_at, updated_at
		FROM ` + constants.BookingsTableName + `
		WHERE user_id = $1
		ORDER BY lower(time_range) DESC
	`

	rows, err := repository.database.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsByUser", err)
	}
	defer rows.Close()

	var bookings []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel, kindLabel string
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID,
			&b.Start, &b.End,
			&statusLabel, &b.HoldExpiresAt, &b.Amount, &b.PaymentID,
			&kindLabel, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListBookingsByUser", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		b.Kind, _ = parseBookingKind(kindLabel)
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsByUser", err)
	}
	return bookings, nil
}
