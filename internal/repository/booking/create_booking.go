package booking

import (
	"context"
	"time"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// exclusionViolationCode is the PostgreSQL SQLSTATE for a rejected EXCLUDE constraint —
// here, an overlapping active booking for the same parking spot.
const exclusionViolationCode = "23P01"

// CreateBooking inserts a new booking and returns it with its generated id and timestamps.
// If the row overlaps an existing active booking for the same parking spot, the EXCLUDE
// constraint on (parking_spot_id, time_range) rejects it; that failure is mapped to a
// domain conflict error.
func (repository *Repository) CreateBooking(ctx context.Context, booking entity.Booking) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		INSERT INTO `+constants.BookingsTableName+`
			(parking_spot_id, user_id, time_range, status, hold_expires_at, amount)
		VALUES ($1, $2, tstzrange($3, $4, '[)'), $5::booking_status, $6, $7)
		RETURNING id, created_at, updated_at
	`, booking.ParkingSpotID, booking.UserID, booking.Start, booking.End, booking.Status.String(), booking.HoldExpiresAt, booking.Amount)

	var id string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &createdAt, &updatedAt); err != nil {
		if pqError, ok := err.(*pq.Error); ok && pqError.Code == exclusionViolationCode {
			return entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking")
		}
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}

	booking.ID = id
	booking.CreatedAt = createdAt
	booking.UpdatedAt = updatedAt
	return booking, nil
}
