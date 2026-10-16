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
	verificationCode *string,
) (entity.Booking, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.ConfirmBooking", err)
	}
	defer func() { _ = transaction.Rollback() }()

	row := transaction.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status                  = $3::booking_status,
			payment_id              = $2,
			paid_at                 = now(),
			owner_response_deadline = $4,
			verification_code       = $5,
			updated_at              = now()
		WHERE id            = $1
		  AND status        = 'PENDING_PAYMENT'
		  AND hold_expires_at > now()
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, payment_id, paid_at,
			kind, verification_code, verification_attempts, verified_at, created_at, updated_at
	`, bookingID, paymentID, status.String(), ownerResponseDeadline, verificationCode)

	var b entity.Booking
	var statusLabel, kindLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID, &b.PaidAt,
		&kindLabel, &b.VerificationCode, &b.VerificationAttempts, &b.VerifiedAt, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	// The payment included the pending balances this booking took on: they're settled.
	if _, err := transaction.ExecContext(ctx, `
		UPDATE `+constants.BookingDebtsTableName+`
		SET    status = 'PAID', paid_at = now()
		WHERE  charged_booking_id = $1 AND status = 'RESERVED'
	`, bookingID); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.ConfirmBooking", err)
	}
	if err := transaction.Commit(); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.ConfirmBooking", err)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	return b, nil
}
