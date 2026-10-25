package booking

import (
	"sort"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

const (
	// latestStartTolerance is how far in the past an hourly booking may still start: the
	// app proposes "now" rounded up, and the driver can take a few minutes to pay for it.
	latestStartTolerance = 15 * time.Minute
	// MinHourlyDuration is the shortest hourly booking (the app's own minimum too).
	MinHourlyDuration = 30 * time.Minute
	// MaxHourlyDuration caps how long one hourly booking can run -- days on end belong to a stay.
	MaxHourlyDuration = 31 * 24 * time.Hour
	// MaxBookingHorizon is how far ahead any booking can start.
	MaxBookingHorizon = 365 * 24 * time.Hour
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
// midnight -- is fully contained in start's day's open hours. Windows that touch (08-12 and
// 12-20) count as one stretch, so a booking from 11 to 13 fits a spot open straight through.
func sliceWithinDayWindows(start, end time.Time, schedule []entity.ScheduleEntry, location *time.Location) bool {
	local := start.In(location)
	dayOfWeek := (int(local.Weekday()) + 6) % 7 // time.Weekday: Sunday=0...Saturday=6, schedule: Monday=0...Sunday=6

	var windows [][2]time.Time
	for _, entry := range schedule {
		if entry.DayOfWeek != dayOfWeek {
			continue
		}
		windows = append(windows, [2]time.Time{
			time.Date(local.Year(), local.Month(), local.Day(), entry.StartHour, 0, 0, 0, location),
			time.Date(local.Year(), local.Month(), local.Day(), entry.EndHour, 0, 0, 0, location),
		})
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i][0].Before(windows[j][0]) })
	var merged [][2]time.Time
	for _, window := range windows {
		if last := len(merged) - 1; last >= 0 && !window[0].After(merged[last][1]) {
			if window[1].After(merged[last][1]) {
				merged[last][1] = window[1]
			}
			continue
		}
		merged = append(merged, window)
	}
	for _, window := range merged {
		if !start.Before(window[0]) && !end.After(window[1]) {
			return true
		}
	}
	return false
}

// checkHourlyRange enforces, from the spot's own rules, when an hourly booking can be: the spot
// takes them, the range isn't already gone (beyond a few minutes), it lasts at least
// MinHourlyDuration and at most MaxHourlyDuration, touches no day the owner closed, and falls
// within the published hours.
func (service *Service) checkHourlyRange(start, end time.Time, rules entity.SpotBookingRules) error {
	if !rules.HourlyEnabled {
		return errors.NewInvalidInput("HOURLY_NOT_OFFERED", "esta cochera no toma reservas por hora")
	}
	if start.Before(service.now().Add(-latestStartTolerance)) {
		return errors.NewInvalidInput("BOOKING_IN_PAST", "ese horario ya pasó: elegí uno desde ahora en adelante")
	}
	duration := end.Sub(start)
	if duration < MinHourlyDuration {
		return errors.NewInvalidInput("BOOKING_TOO_SHORT", "la reserva tiene que durar al menos 30 minutos")
	}
	if duration > MaxHourlyDuration {
		return errors.NewInvalidInput("BOOKING_TOO_LONG", "una reserva por hora puede durar hasta 31 días: para más tiempo, reservá una estadía")
	}
	if entity.OverlapsDateRanges(rules.BlockedDates, start, end, service.location) {
		return errors.NewInvalidInput("SPOT_DAY_BLOCKED", "el propietario cerró la cochera ese día")
	}
	if !isWithinPublishedHours(start, end, rules.Schedule, service.location) {
		return errors.NewInvalidInput("OUTSIDE_PUBLISHED_HOURS", "la cochera no está abierta en todo ese horario: elegí otro dentro de su horario")
	}
	return nil
}

// checkHorizon refuses a booking that starts more than MaxBookingHorizon ahead -- whatever its
// kind -- so nobody can tie up a spot years from now.
func (service *Service) checkHorizon(start time.Time) error {
	if start.After(service.now().Add(MaxBookingHorizon)) {
		return errors.NewInvalidInput("BOOKING_TOO_FAR", "solo se puede reservar hasta un año por delante")
	}
	return nil
}
