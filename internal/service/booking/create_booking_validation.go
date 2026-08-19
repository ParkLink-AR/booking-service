package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

// isWithinPublishedHours reports whether [start, end) is fully contained in at least one
// window of availabilityBase.
func isWithinPublishedHours(start, end time.Time, availabilityBase *entity.AvailabilityBase) bool {
	for _, day := range availabilityBase.Days {
		for _, window := range day.Windows {
			if !start.Before(window.Start) && !end.After(window.End) {
				return true
			}
		}
	}
	return false
}

// hasOverlap reports whether [start, end) overlaps any of activeBookings.
func hasOverlap(activeBookings []entity.TimeWindow, start, end time.Time) bool {
	for _, booking := range activeBookings {
		if booking.Start.Before(end) && booking.End.After(start) {
			return true
		}
	}
	return false
}
