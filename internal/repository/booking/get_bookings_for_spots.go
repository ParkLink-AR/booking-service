package booking

import (
	"context"
	"time"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetBookingsForSpotsInRange returns every booking, of any status, for any of
// parkingSpotIDs that would collide with [start, end) once the grace period is counted (see
// entity.OccupancyWindow), i.e. whose range overlaps [start-grace, end+grace). It does not
// decide which statuses occupy the spot — the caller applies that business rule.
func (repository *Repository) GetBookingsForSpotsInRange(
	ctx context.Context,
	parkingSpotIDs []string,
	start, end time.Time,
) ([]entity.SpotBooking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT parking_spot_id, status, hold_expires_at
		FROM `+constants.BookingsTableName+`
		WHERE parking_spot_id = ANY($1::uuid[])
		  AND time_range && tstzrange($2, $3, '[)')
	`, pq.Array(parkingSpotIDs), start.Add(-entity.GracePeriod), end.Add(entity.GracePeriod))
	if err != nil {
		return nil, errors.NewInternalError("booking.GetBookingsForSpotsInRange", err)
	}
	defer rows.Close()

	var bookings []entity.SpotBooking
	for rows.Next() {
		var parkingSpotID, statusLabel string
		var holdExpiresAt *time.Time
		if err := rows.Scan(&parkingSpotID, &statusLabel, &holdExpiresAt); err != nil {
			return nil, errors.NewInternalError("booking.GetBookingsForSpotsInRange", err)
		}
		status, _ := enum.ParseBookingStatus(statusLabel)
		bookings = append(bookings, entity.SpotBooking{
			ParkingSpotID: parkingSpotID,
			Status:        status,
			HoldExpiresAt: holdExpiresAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.GetBookingsForSpotsInRange", err)
	}
	return bookings, nil
}
