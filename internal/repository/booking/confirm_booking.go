package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ConfirmBooking atomically transitions a booking from PENDING_PAYMENT to either CONFIRMED
// (instant-booking spot) or PENDING_OWNER_APPROVAL (the owner decides later, see
// ApproveBooking/RejectBooking), and records the Mercado Pago payment_id. It only updates
// rows that are still in PENDING_PAYMENT and whose hold has not yet expired, so a second
// caller racing to confirm the same booking will find 0 rows and get a NotFound error.
func (repository *Repository) ConfirmBooking(
	ctx context.Context,
	bookingID, paymentID string,
	status enum.BookingStatus,
	ownerResponseDeadline *time.Time,
) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status                  = $3::booking_status,
			payment_id              = $2,
			owner_response_deadline = $4,
			updated_at              = now()
		WHERE id            = $1
		  AND status        = 'PENDING_PAYMENT'
		  AND hold_expires_at > now()
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, payment_id,
			kind, created_at, updated_at
	`, bookingID, paymentID, status.String(), ownerResponseDeadline)

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
