package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ConfirmBooking atomically transitions a booking from PENDING_PAYMENT → CONFIRMED and
// records the Mercado Pago payment_id.  It only updates rows that are still in
// PENDING_PAYMENT and whose hold has not yet expired, so a second caller racing to
// confirm the same booking will find 0 rows and get a NotFound error.
func (repository *Repository) ConfirmBooking(ctx context.Context, bookingID, paymentID string) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status     = 'CONFIRMED'::booking_status,
			payment_id = $2,
			updated_at = now()
		WHERE id            = $1
		  AND status        = 'PENDING_PAYMENT'
		  AND hold_expires_at > now()
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, amount, payment_id,
			created_at, updated_at
	`, bookingID, paymentID)

	var b entity.Booking
	var statusLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.Amount, &b.PaymentID,
		&b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	return b, nil
}
