package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetBookableSpots returns the subset of parkingSpotIDs that have no booking occupying
// [start, end). It does not consider a spot's published hours — only whether an existing
// booking already occupies the requested range. Checking published hours is out of scope
// here: parking-service does not yet expose structured schedule data for this service to
// validate against.
func (service *Service) GetBookableSpots(
	ctx context.Context,
	parkingSpotIDs []string,
	start, end time.Time,
) ([]string, error) {
	if !start.Before(end) {
		return nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "start must be before end")
	}
	if len(parkingSpotIDs) == 0 {
		return nil, nil
	}

	bookings, err := service.bookingRepository.GetBookingsForSpotsInRange(ctx, parkingSpotIDs, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}

	return excludeIDs(parkingSpotIDs, occupiedSpotIDs(bookings)), nil
}

// occupiedSpotIDs returns the distinct parking spot IDs with at least one booking that
// currently occupies the spot: CONFIRMED, or PENDING_PAYMENT with an unexpired hold.
func occupiedSpotIDs(bookings []entity.SpotBooking) []string {
	occupied := make(map[string]struct{})
	for _, booking := range bookings {
		if isActive(booking.Status, booking.HoldExpiresAt) {
			occupied[booking.ParkingSpotID] = struct{}{}
		}
	}

	ids := make([]string, 0, len(occupied))
	for id := range occupied {
		ids = append(ids, id)
	}
	return ids
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

// excludeIDs returns the elements of all that are not present in exclude, preserving all's order.
func excludeIDs(all, exclude []string) []string {
	excluded := make(map[string]struct{}, len(exclude))
	for _, id := range exclude {
		excluded[id] = struct{}{}
	}

	bookable := make([]string, 0, len(all))
	for _, id := range all {
		if _, found := excluded[id]; !found {
			bookable = append(bookable, id)
		}
	}
	return bookable
}
