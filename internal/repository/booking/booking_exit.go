package booking

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

const bookingExitColumns = `
	booking_id, exit_code, exit_attempts, exited_at, COALESCE(closed_by, ''),
	overstay_minutes, charged_minutes, overstay_amount, overstay_owner_amount`

func scanBookingExit(scanner rowScanner) (entity.BookingExit, error) {
	var exit entity.BookingExit
	if err := scanner.Scan(
		&exit.BookingID, &exit.Code, &exit.Attempts, &exit.ExitedAt, &exit.ClosedBy,
		&exit.Overstay.Minutes, &exit.Overstay.ChargedMinutes, &exit.Overstay.Amount, &exit.Overstay.OwnerAmount,
	); err != nil {
		return entity.BookingExit{}, err
	}
	return exit, nil
}

// FindExit returns bookingID's exit, or nil when it has none yet (its driver never asked for
// the code).
func (repository *Repository) FindExit(ctx context.Context, bookingID string) (*entity.BookingExit, error) {
	exit, err := scanBookingExit(repository.database.QueryRowContext(ctx, `
		SELECT `+bookingExitColumns+`
		FROM `+constants.BookingExitsTableName+`
		WHERE booking_id = $1
	`, bookingID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.NewInternalError("booking.FindExit", err)
	}
	return &exit, nil
}

// EnsureExit creates bookingID's exit with code, unless it already has one, and returns it --
// whichever code it ended up with, so two requests racing to create it agree on the same one.
func (repository *Repository) EnsureExit(ctx context.Context, bookingID, code string) (entity.BookingExit, error) {
	if _, err := repository.database.ExecContext(ctx, `
		INSERT INTO `+constants.BookingExitsTableName+` (booking_id, exit_code)
		VALUES ($1, $2)
		ON CONFLICT (booking_id) DO NOTHING
	`, bookingID, code); err != nil {
		return entity.BookingExit{}, errors.NewInternalError("booking.EnsureExit", err)
	}
	exit, err := repository.FindExit(ctx, bookingID)
	if err != nil {
		return entity.BookingExit{}, err
	}
	if exit == nil {
		return entity.BookingExit{}, errors.NewNotFound("booking", bookingID)
	}
	return *exit, nil
}

// VerifyExitCode compares code against the one the driver was shown, in constant time, like
// VerifyBookingCode does for the arrival code: a wrong one counts an attempt, and past
// maxVerificationAttempts no more are compared. Returns nil on a match; it is CloseExit that
// records the departure.
func (repository *Repository) VerifyExitCode(ctx context.Context, bookingID, code string) error {
	exit, err := repository.FindExit(ctx, bookingID)
	if err != nil {
		return err
	}
	// No exit yet means the driver never looked at the code, so nobody has it.
	if exit == nil {
		return errors.NewInvalidInput("INVALID_CODE", "código incorrecto")
	}
	if exit.Closed() {
		return errors.NewConflict("la salida de esta reserva ya está registrada")
	}
	if exit.Attempts >= maxVerificationAttempts {
		return errors.NewConflict("se alcanzó el máximo de intentos para este código")
	}
	if subtle.ConstantTimeCompare([]byte(exit.Code), []byte(code)) != 1 {
		if _, err := repository.database.ExecContext(ctx, `
			UPDATE `+constants.BookingExitsTableName+`
			SET    exit_attempts = exit_attempts + 1, updated_at = now()
			WHERE  booking_id = $1
		`, bookingID); err != nil {
			return errors.NewInternalError("booking.VerifyExitCode", err)
		}
		return errors.NewInvalidInput("INVALID_CODE", "código incorrecto")
	}
	return nil
}

// CloseExit records that bookingID's vehicle left at exitedAt (by closedBy), with what staying
// past the end amounts to, and completes the booking. Returns it as it is now. A booking
// closed twice is a conflict: whoever got there first decided when the vehicle left.
//
// What the overstay costs is settled in the same transaction: the driver owes it as a debt that
// rides on their next reservation (see BookingDebt), and ParkLink owes the spot's owner
// (ownerID) their share as a payout of its own -- ParkLink pays it out of its pocket and
// recovers it from the driver.
func (repository *Repository) CloseExit(
	ctx context.Context, bookingID string, exitedAt time.Time, closedBy string, overstay entity.Overstay, ownerID string,
) (entity.Booking, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CloseExit", err)
	}
	defer func() { _ = transaction.Rollback() }()

	result, err := transaction.ExecContext(ctx, `
		UPDATE `+constants.BookingExitsTableName+`
		SET    exited_at             = $2,
		       closed_by             = $3,
		       overstay_minutes      = $4,
		       charged_minutes       = $5,
		       overstay_amount       = $6,
		       overstay_owner_amount = $7,
		       updated_at            = now()
		WHERE  booking_id = $1 AND exited_at IS NULL
	`, bookingID, exitedAt, closedBy, overstay.Minutes, overstay.ChargedMinutes, overstay.Amount, overstay.OwnerAmount)
	if err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CloseExit", err)
	}
	if closed, _ := result.RowsAffected(); closed == 0 {
		return entity.Booking{}, errors.NewConflict("la salida de esta reserva ya está registrada")
	}
	if _, err := transaction.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    status = 'COMPLETED'::booking_status, updated_at = now()
		WHERE  id = $1 AND status IN ('CONFIRMED', 'IN_PROGRESS')
	`, bookingID); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CloseExit", err)
	}
	if overstay.ChargedMinutes > 0 && overstay.Amount > 0 {
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO `+constants.BookingDebtsTableName+` (user_id, source_booking_id, minutes, amount)
			SELECT user_id, id, $2, $3 FROM `+constants.BookingsTableName+` WHERE id = $1
			ON CONFLICT (source_booking_id) DO NOTHING
		`, bookingID, overstay.ChargedMinutes, overstay.Amount); err != nil {
			return entity.Booking{}, errors.NewInternalError("booking.CloseExit", err)
		}
	}
	if overstay.ChargedMinutes > 0 && overstay.OwnerAmount > 0 && ownerID != "" {
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO `+constants.PayoutsTableName+`
				(booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount, kind)
			SELECT id, $2, COALESCE(payment_id, ''), $3, 0, $3, 'OVERSTAY' FROM `+constants.BookingsTableName+` WHERE id = $1
			ON CONFLICT (booking_id, kind) DO NOTHING
		`, bookingID, ownerID, overstay.OwnerAmount); err != nil {
			return entity.Booking{}, errors.NewInternalError("booking.CloseExit", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CloseExit", err)
	}
	return repository.GetByID(ctx, bookingID)
}
