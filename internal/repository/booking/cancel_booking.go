package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CancelBooking transitions a booking from CONFIRMED or PENDING_OWNER_APPROVAL to
// CANCELLED. The time-window guard (must cancel at least 1 hour before start) is
// enforced by the use case, not here. Returns the full row so the caller can trigger a
// refund.
func (repository *Repository) CancelBooking(ctx context.Context, bookingID string) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status     = 'CANCELLED'::booking_status,
			updated_at = now()
		WHERE id     = $1
		  AND status IN ('CONFIRMED', 'PENDING_OWNER_APPROVAL')
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''), paid_at,
			kind, created_at, updated_at
	`, bookingID)

	var b entity.Booking
	var statusLabel, kindLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID, &b.PaidAt,
		&kindLabel, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	return b, nil
}
