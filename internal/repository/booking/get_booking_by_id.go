package booking

import (
	"context"
	"database/sql"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetBookingByID returns a single booking by id.
func (repository *Repository) GetBookingByID(ctx context.Context, bookingID string) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		SELECT
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, amount, payment_id,
			created_at, updated_at
		FROM `+constants.BookingsTableName+`
		WHERE id = $1
	`, bookingID)

	var b entity.Booking
	var statusLabel string
	var paymentID sql.NullString
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.Amount, &paymentID,
		&b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.PaymentID = paymentID.String
	return b, nil
}
