package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

// isWithinPublishedHours reports whether [start, end) is fully contained in one of
// schedule's windows for the day of week start falls on, evaluated in location's local
// time. Assumes start and end fall on the same local calendar day -- every caller only
// ever offers same-day booking slots.
func isWithinPublishedHours(start, end time.Time, schedule []entity.ScheduleEntry, location *time.Location) bool {
	local := start.In(location)
	dayOfWeek := (int(local.Weekday()) + 6) % 7 // time.Weekday: Sunday=0...Saturday=6, schedule: Monday=0...Sunday=6

	for _, entry := range schedule {
		if entry.DayOfWeek != dayOfWeek {
			continue
		}
		windowStart := time.Date(local.Year(), local.Month(), local.Day(), entry.StartHour, 0, 0, 0, location)
		windowEnd := time.Date(local.Year(), local.Month(), local.Day(), entry.EndHour, 0, 0, 0, location)
		if !start.Before(windowStart) && !end.After(windowEnd) {
			return true
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
