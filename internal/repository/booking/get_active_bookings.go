package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetActiveBookingsInRange returns the time windows of bookings that occupy availability
// (CONFIRMED, or PENDING_PAYMENT with an unexpired hold) and overlap [from, to).
func (repository *Repository) GetActiveBookingsInRange(
	ctx context.Context,
	parkingSpotID string,
	from, to time.Time,
) ([]entity.TimeWindow, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT lower(time_range), upper(time_range)
		FROM `+constants.BookingsTableName+`
		WHERE parking_spot_id = $1
		  AND (
		    status = 'CONFIRMED'
		    OR (status = 'PENDING_PAYMENT' AND hold_expires_at > now())
		  )
		  AND time_range && tstzrange($2, $3, '[)')
	`, parkingSpotID, from, to)
	if err != nil {
		return nil, errors.NewInternalError("booking.GetActiveBookingsInRange", err)
	}
	defer rows.Close()

	var activeBookings []entity.TimeWindow
	for rows.Next() {
		var start, end time.Time
		if err := rows.Scan(&start, &end); err != nil {
			return nil, errors.NewInternalError("booking.GetActiveBookingsInRange", err)
		}
		activeBookings = append(activeBookings, entity.TimeWindow{Start: start, End: end})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.GetActiveBookingsInRange", err)
	}
	return activeBookings, nil
}
