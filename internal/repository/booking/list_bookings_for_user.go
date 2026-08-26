package booking

import (
	"context"
	"database/sql"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListBookingsForUser returns every booking made by userID (any parking spot, any status),
// most recent first.
func (repository *Repository) ListBookingsForUser(ctx context.Context, userID string) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, amount, payment_id,
			created_at, updated_at
		FROM `+constants.BookingsTableName+`
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsForUser", err)
	}
	defer rows.Close()

	var bookings []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel string
		var paymentID sql.NullString
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID,
			&b.Start, &b.End,
			&statusLabel, &b.HoldExpiresAt, &b.Amount, &paymentID,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListBookingsForUser", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		b.PaymentID = paymentID.String
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsForUser", err)
	}
	return bookings, nil
}
