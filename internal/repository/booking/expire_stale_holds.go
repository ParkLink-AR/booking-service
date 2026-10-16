package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ExpireStaleHolds transitions every PENDING_PAYMENT booking whose hold_expires_at
// is in the past to EXPIRED. This releases the GIST EXCLUDE constraint so those
// time slots become bookable again without the service needing to restart.
// Returns the number of rows updated.
func (repository *Repository) ExpireStaleHolds(ctx context.Context) (int64, error) {
	// The pending balances an expired hold had taken on go back to the driver in the same
	// statement, so there is no moment where they are attached to a booking that can't be paid.
	var expired int64
	if err := repository.database.QueryRowContext(ctx, `
		WITH expired AS (
			UPDATE `+constants.BookingsTableName+`
			SET    status     = 'EXPIRED',
			       updated_at = now()
			WHERE  status          = 'PENDING_PAYMENT'
			  AND  hold_expires_at IS NOT NULL
			  AND  hold_expires_at < now()
			RETURNING id
		), released AS (
			UPDATE `+constants.BookingDebtsTableName+`
			SET    status = 'PENDING', charged_booking_id = NULL
			WHERE  status = 'RESERVED' AND charged_booking_id IN (SELECT id FROM expired)
		)
		SELECT count(*) FROM expired
	`).Scan(&expired); err != nil {
		return 0, errors.NewInternalError("booking.ExpireStaleHolds", err)
	}
	return expired, nil
}
