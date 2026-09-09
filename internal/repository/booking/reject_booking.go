package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// RejectBooking transitions a booking from PENDING_OWNER_APPROVAL to CANCELLED. Returns
// the full row (payment_id included) so the caller can trigger a refund. Ownership of the
// parking spot is verified by the usecase, not here.
func (repository *Repository) RejectBooking(ctx context.Context, bookingID string) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status                  = 'CANCELLED'::booking_status,
			owner_response_deadline = NULL,
			updated_at              = now()
		WHERE id     = $1
		  AND status = 'PENDING_OWNER_APPROVAL'
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''),
			kind, created_at, updated_at
	`, bookingID)

	var b entity.Booking
	var statusLabel, kindLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID,
		&kindLabel, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	return b, nil
}
