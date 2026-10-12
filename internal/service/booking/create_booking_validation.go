package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

// isWithinPublishedHours reports whether [start, end) is fully covered by the spot's
// published hours, evaluated in location's local time. A range that crosses midnight (or
// spans several days) is split at each local midnight and every day's slice must fit inside
// one of that day's schedule windows -- so a spot open 24 hours on consecutive days can be
// booked straight through the night, while a closed or short day in the middle rejects it.
func isWithinPublishedHours(start, end time.Time, schedule []entity.ScheduleEntry, location *time.Location) bool {
	if !start.Before(end) {
		return false
	}

	cursor := start.In(location)
	end = end.In(location)
	for cursor.Before(end) {
		nextMidnight := time.Date(cursor.Year(), cursor.Month(), cursor.Day()+1, 0, 0, 0, 0, location)
		sliceEnd := end
		if nextMidnight.Before(end) {
			sliceEnd = nextMidnight
		}
		if !sliceWithinDayWindows(cursor, sliceEnd, schedule, location) {
			return false
		}
		cursor = nextMidnight
	}
	return true
}

// sliceWithinDayWindows reports whether [start, end) -- which must not cross a local
// midnight -- is fully contained in one of schedule's windows for start's day of week.
func sliceWithinDayWindows(start, end time.Time, schedule []entity.ScheduleEntry, location *time.Location) bool {
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
