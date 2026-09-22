package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ApproveBooking transitions a booking from PENDING_OWNER_APPROVAL to CONFIRMED, stamping
// a fresh in-person verification code (see entity.GenerateVerificationCode). Ownership of
// the parking spot is verified by the usecase, not here.
func (repository *Repository) ApproveBooking(ctx context.Context, bookingID, verificationCode string) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status                  = 'CONFIRMED'::booking_status,
			owner_response_deadline = NULL,
			verification_code       = $2,
			updated_at              = now()
		WHERE id     = $1
		  AND status = 'PENDING_OWNER_APPROVAL'
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''), paid_at,
			kind, verification_code, verification_attempts, verified_at, created_at, updated_at
	`, bookingID, verificationCode)

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
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	return b, nil
}
