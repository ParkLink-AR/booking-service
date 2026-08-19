package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListBookings returns bookings for parkingSpotID ordered by start time. from and to,
// when both nil, apply no time filter; when either is set, only bookings overlapping the
// half-open range are returned (a nil bound is unbounded on that side). status, when set,
// restricts results to that single booking status.
func (repository *Repository) ListBookings(
	ctx context.Context,
	parkingSpotID string,
	from, to *time.Time,
	status *enum.BookingStatus,
) ([]entity.BookingWindow, error) {
	query := `
		SELECT lower(time_range), upper(time_range), status
		FROM ` + constants.BookingsTableName + `
		WHERE parking_spot_id = $1
	`
	args := []any{parkingSpotID}

	if from != nil || to != nil {
		args = append(args, from, to)
		query += fmt.Sprintf(" AND time_range && tstzrange($%d, $%d, '[)')", len(args)-1, len(args))
	}
	if status != nil {
		args = append(args, status.String())
		query += fmt.Sprintf(" AND status = $%d::booking_status", len(args))
	}
	query += " ORDER BY lower(time_range)"

	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListBookings", err)
	}
	defer rows.Close()

	var bookingWindows []entity.BookingWindow
	for rows.Next() {
		var start, end time.Time
		var statusLabel string
		if err := rows.Scan(&start, &end, &statusLabel); err != nil {
			return nil, errors.NewInternalError("booking.ListBookings", err)
		}
		bookingStatus, _ := enum.ParseBookingStatus(statusLabel)
		bookingWindows = append(bookingWindows, entity.BookingWindow{Start: start, End: end, Status: bookingStatus})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListBookings", err)
	}
	return bookingWindows, nil
}
