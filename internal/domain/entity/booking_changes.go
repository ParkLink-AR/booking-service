package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// BookingExtension is the short view of a booking's extension shown on the booking itself:
// which booking continues it, until when, and whether it's paid yet.
type BookingExtension struct {
	ID     string
	End    time.Time
	Status enum.BookingStatus
}

// DateRange is a run of whole days, From and To inclusive, as YYYY-MM-DD in the service's
// location -- e.g. the days an owner closed their spot by hand.
type DateRange struct {
	From string
	To   string
}

// Window is the instants the range covers: from 00:00 of From to 00:00 of the day after To.
func (dateRange DateRange) Window(location *time.Location) (TimeWindow, bool) {
	from, err := time.ParseInLocation("2006-01-02", dateRange.From, location)
	if err != nil {
		return TimeWindow{}, false
	}
	to, err := time.ParseInLocation("2006-01-02", dateRange.To, location)
	if err != nil {
		return TimeWindow{}, false
	}
	return TimeWindow{Start: from, End: to.AddDate(0, 0, 1)}, true
}

// OverlapsDateRanges reports whether [start, end) touches any of ranges.
func OverlapsDateRanges(ranges []DateRange, start, end time.Time, location *time.Location) bool {
	for _, dateRange := range ranges {
		window, ok := dateRange.Window(location)
		if ok && start.Before(window.End) && window.Start.Before(end) {
			return true
		}
	}
	return false
}

// SpotBookingRules is what deciding on an hourly booking needs to know about its spot:
// whether it takes new bookings, its weekly hours, how many cars fit, and the days its
// owner closed.
type SpotBookingRules struct {
	Active        bool
	HourlyEnabled bool
	Schedule      []ScheduleEntry
	Capacity      int
	BlockedDates  []DateRange
}

// ExtensionOption is one way to extend a booking: until End, for Amount (what the driver
// pays, centavos) of which OwnerAmount is the owner's.
type ExtensionOption struct {
	End         time.Time
	Amount      int
	OwnerAmount int
}

// OwnerCancellationStats is how often an owner called off bookings they had accepted.
type OwnerCancellationStats struct {
	Total      int
	Last90Days int
}
