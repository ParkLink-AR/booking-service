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
	result, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    status     = 'EXPIRED',
		       updated_at = now()
		WHERE  status          = 'PENDING_PAYMENT'
		  AND  hold_expires_at IS NOT NULL
		  AND  hold_expires_at < now()
	`)
	if err != nil {
		return 0, errors.NewInternalError("booking.ExpireStaleHolds", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, errors.NewInternalError("booking.ExpireStaleHolds.RowsAffected", err)
	}
	return n, nil
}
