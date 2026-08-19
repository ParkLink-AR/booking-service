package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

// groupByDay buckets consecutive bookingWindows that fall on the same calendar day, in
// location. It assumes bookingWindows is already ordered by Start ascending, so windows for
// the same day are always adjacent. Only days with at least one booking appear.
func groupByDay(bookingWindows []entity.BookingWindow, location *time.Location) []entity.DayBookings {
	var days []entity.DayBookings

	for _, window := range bookingWindows {
		local := window.Start.In(location)
		date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)

		if len(days) > 0 && days[len(days)-1].Date.Equal(date) {
			lastDay := &days[len(days)-1]
			lastDay.Bookings = append(lastDay.Bookings, window)
			continue
		}
		days = append(days, entity.DayBookings{Date: date, Bookings: []entity.BookingWindow{window}})
	}

	return days
}
