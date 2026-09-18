package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ExpireCompletedBookings transitions every CONFIRMED or IN_PROGRESS booking whose time
// window has already ended to COMPLETED. This is what makes a stay "over" -- it's the
// trigger the review prompts key off of. IN_PROGRESS bookings complete the same way as
// CONFIRMED ones regardless of whether the driver's in-person verification code was ever
// entered -- verified_at (untouched here) is the only record of that. Returns the number
// of rows updated.
func (repository *Repository) ExpireCompletedBookings(ctx context.Context) (int64, error) {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    status     = 'COMPLETED'::booking_status,
		       updated_at = now()
		WHERE  status IN ('CONFIRMED', 'IN_PROGRESS')
		  AND  upper(time_range) < now()
	`)
	if err != nil {
		return 0, errors.NewInternalError("booking.ExpireCompletedBookings", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, errors.NewInternalError("booking.ExpireCompletedBookings.RowsAffected", err)
	}
	return n, nil
}
