package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CancelBooking transitions a booking from CONFIRMED or PENDING_OWNER_APPROVAL to
// CANCELLED, as the driver's doing: reason is DRIVER_CANCELLED, or DRIVER_LATE_CANCELLED
// when only refundedAmount (centavos) of it goes back to them. The time-window rules are
// enforced by the use case, not here. Returns the full row so the caller can trigger a
// refund. A full refund also puts back the debts the booking's payment had settled.
func (repository *Repository) CancelBooking(ctx context.Context, bookingID, reason string, refundedAmount int) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status              = 'CANCELLED'::booking_status,
			cancellation_reason = $2,
			refunded_amount     = $3,
			updated_at          = now()
		WHERE id     = $1
		  AND status IN ('CONFIRMED', 'PENDING_OWNER_APPROVAL')
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''), paid_at,
			cancellation_reason, kind, created_at, updated_at
	`, bookingID, reason, refundedAmount)

	var b entity.Booking
	var statusLabel, kindLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID, &b.PaidAt,
		&b.CancellationReason, &kindLabel, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	// Given back in full, the payment's debt part goes back too: the driver owes it again. A
	// partial refund (a late cancellation) only returns the booking's own share.
	if refundedAmount == 0 {
		if err := repository.ReleaseDebts(ctx, bookingID); err != nil {
			return entity.Booking{}, err
		}
	}
	return b, nil
}
