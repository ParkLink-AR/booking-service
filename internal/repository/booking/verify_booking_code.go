package booking

import (
	"context"
	"crypto/subtle"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// maxVerificationAttempts caps how many wrong codes a caller may enter against a single
// booking before verification is refused outright, regardless of whether the next guess
// would be correct. Chosen for a 4-digit code entered by a human standing in front of the
// driver, not for resisting a scripted brute force.
const maxVerificationAttempts = 5

// VerifyBookingCode compares code against the CONFIRMED booking's stored verification
// code, in constant time (crypto/subtle) so response timing can't leak how many digits
// matched. On a match, transitions the booking to IN_PROGRESS and stamps verified_at. On a
// mismatch, increments verification_attempts and returns an InvalidInputError; once the
// attempt limit is reached, returns a ConflictError instead without comparing further.
func (repository *Repository) VerifyBookingCode(ctx context.Context, bookingID, code string) (entity.Booking, error) {
	booking, err := repository.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}
	if booking.Status != enum.BookingStatusConfirmed {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	if booking.VerificationAttempts >= maxVerificationAttempts {
		return entity.Booking{}, errors.NewConflict("se alcanzó el máximo de intentos para este código")
	}
	if booking.VerificationCode == nil ||
		subtle.ConstantTimeCompare([]byte(*booking.VerificationCode), []byte(code)) != 1 {
		if err := repository.incrementVerificationAttempts(ctx, bookingID); err != nil {
			return entity.Booking{}, err
		}
		return entity.Booking{}, errors.NewInvalidInput("INVALID_CODE", "código incorrecto")
	}

	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    status      = 'IN_PROGRESS'::booking_status,
		       verified_at = now(),
		       updated_at  = now()
		WHERE  id = $1 AND status = 'CONFIRMED'
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''), paid_at,
			kind, verification_code, verification_attempts, verified_at, created_at, updated_at
	`, bookingID)

	var b entity.Booking
	var statusLabel, kindLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID, &b.PaidAt,
		&kindLabel, &b.VerificationCode, &b.VerificationAttempts, &b.VerifiedAt, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		// Lost a race with another transition away from CONFIRMED between the check
		// above and this UPDATE (e.g. the completion sweep). Rare enough not to
		// warrant a retry loop for a human-paced, in-person action.
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	// The driver arrived: the extensions already paid for continue the same stay, so they
	// count as verified too (best effort -- a failure only leaves them to an admin's review).
	_ = repository.PropagateVerificationToExtensions(ctx, b.ID)
	return b, nil
}

func (repository *Repository) incrementVerificationAttempts(ctx context.Context, bookingID string) error {
	if _, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    verification_attempts = verification_attempts + 1,
		       updated_at            = now()
		WHERE  id = $1
	`, bookingID); err != nil {
		return errors.NewInternalError("booking.incrementVerificationAttempts", err)
	}
	return nil
}
