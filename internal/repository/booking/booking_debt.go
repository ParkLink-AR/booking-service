package booking

import (
	"context"
	"strconv"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// maxListedDebts caps how many debts one listing returns, newest first: a driver rarely has more
// than a handful, and the paid ones are history.
const maxListedDebts = 50

const bookingDebtColumns = `
	d.id, d.user_id, d.source_booking_id, d.charged_booking_id, d.minutes, d.amount, d.status,
	d.created_at, d.paid_at,
	lower(b.time_range), upper(b.time_range), b.parking_spot_id`

func scanBookingDebt(scanner rowScanner) (entity.BookingDebt, error) {
	var debt entity.BookingDebt
	if err := scanner.Scan(
		&debt.ID, &debt.UserID, &debt.SourceBookingID, &debt.ChargedBookingID, &debt.Minutes, &debt.Amount, &debt.Status,
		&debt.CreatedAt, &debt.PaidAt,
		&debt.SourceStart, &debt.SourceEnd, &debt.ParkingSpotID,
	); err != nil {
		return entity.BookingDebt{}, err
	}
	return debt, nil
}

func (repository *Repository) queryDebts(ctx context.Context, operation, where string, args ...any) ([]entity.BookingDebt, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT `+bookingDebtColumns+`
		FROM `+constants.BookingDebtsTableName+` d
		JOIN `+constants.BookingsTableName+` b ON b.id = d.source_booking_id
		WHERE `+where+`
		ORDER BY d.created_at DESC
		LIMIT `+strconv.Itoa(maxListedDebts), args...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}
	defer rows.Close()

	var debts []entity.BookingDebt
	for rows.Next() {
		debt, err := scanBookingDebt(rows)
		if err != nil {
			return nil, errors.NewInternalError(operation, err)
		}
		debts = append(debts, debt)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}
	return debts, nil
}

// ListDebtsByUser returns userID's debts, newest first: the ones still owed and the ones they
// already paid.
func (repository *Repository) ListDebtsByUser(ctx context.Context, userID string) ([]entity.BookingDebt, error) {
	return repository.queryDebts(ctx, "booking.ListDebtsByUser", `d.user_id = $1`, userID)
}

// ListDebtsChargedTo returns the debts bookingID's payment includes.
func (repository *Repository) ListDebtsChargedTo(ctx context.Context, bookingID string) ([]entity.BookingDebt, error) {
	return repository.queryDebts(ctx, "booking.ListDebtsChargedTo", `d.charged_booking_id = $1`, bookingID)
}

// ListDebtsOfBooking returns every debt bookingID is part of, either because the driver
// overstayed it or because its payment included one: what support looks at on the booking.
func (repository *Repository) ListDebtsOfBooking(ctx context.Context, bookingID string) ([]entity.BookingDebt, error) {
	return repository.queryDebts(ctx, "booking.ListDebtsOfBooking", `d.source_booking_id = $1 OR d.charged_booking_id = $1`, bookingID)
}

// ReleaseDebts puts back to PENDING every debt bookingID's payment included, because that payment
// was given back in full (or never completed): the driver owes them again and their next booking
// takes them on. A partial refund of the booking's own share doesn't call this -- the debt part of
// the payment stays paid.
func (repository *Repository) ReleaseDebts(ctx context.Context, bookingID string) error {
	if _, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingDebtsTableName+`
		SET    status = 'PENDING', charged_booking_id = NULL, paid_at = NULL
		WHERE  charged_booking_id = $1 AND status IN ('RESERVED', 'PAID')
	`, bookingID); err != nil {
		return errors.NewInternalError("booking.ReleaseDebts", err)
	}
	return nil
}

// releaseDebtsOf is ReleaseDebts for several bookings at once, for the sweeps.
func (repository *Repository) releaseDebtsOf(ctx context.Context, operation string, bookingIDs []string) error {
	if len(bookingIDs) == 0 {
		return nil
	}
	if _, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingDebtsTableName+`
		SET    status = 'PENDING', charged_booking_id = NULL, paid_at = NULL
		WHERE  charged_booking_id::text = ANY($1) AND status IN ('RESERVED', 'PAID')
	`, pq.Array(bookingIDs)); err != nil {
		return errors.NewInternalError(operation, err)
	}
	return nil
}
