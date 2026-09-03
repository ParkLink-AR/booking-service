package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetBookableSpots returns the subset of spots that still have room in [start, end) --
// i.e. fewer active bookings overlapping the range than the spot's capacity. A capacity-1
// spot behaves as before ("no active booking"). This is a coarse search pre-filter: it
// counts active bookings that overlap the range, not the precise per-instant concurrency
// (that finer check lives in CreateBooking and in parking-service's availability). It also
// does not consider a spot's published hours -- parking-service already has that locally.
func (service *Service) GetBookableSpots(
	ctx context.Context,
	spots []entity.SpotCapacity,
	start, end time.Time,
) ([]string, error) {
	if !start.Before(end) {
		return nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "start must be before end")
	}
	if len(spots) == 0 {
		return nil, nil
	}

	ids := make([]string, len(spots))
	for i, spot := range spots {
		ids[i] = spot.ID
	}

	bookings, err := service.bookingRepository.GetBookingsForSpotsInRange(ctx, ids, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}

	activeBySpot := make(map[string]int)
	for _, booking := range bookings {
		if isActive(booking.Status, booking.HoldExpiresAt) {
			activeBySpot[booking.ParkingSpotID]++
		}
	}

	bookable := make([]string, 0, len(spots))
	for _, spot := range spots {
		capacity := spot.Capacity
		if capacity < 1 {
			capacity = 1
		}
		if activeBySpot[spot.ID] < capacity {
			bookable = append(bookable, spot.ID)
		}
	}
	return bookable, nil
}

// isActive reports whether a booking with the given status currently occupies its spot:
// CONFIRMED, or PENDING_PAYMENT with an unexpired hold.
func isActive(status enum.BookingStatus, holdExpiresAt *time.Time) bool {
	switch status {
	case enum.BookingStatusConfirmed:
		return true
	case enum.BookingStatusPendingPayment:
		return holdExpiresAt != nil && holdExpiresAt.After(time.Now())
	default:
		return false
	}
}
