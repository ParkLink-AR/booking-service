package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// BookingWindow is a single reservation's occupied time range and status, as returned by
// the booking-listing flow.
type BookingWindow struct {
	Start  time.Time
	End    time.Time
	Status enum.BookingStatus
}

// DayBookings groups the bookings that start on the same calendar day.
type DayBookings struct {
	Date     time.Time
	Bookings []BookingWindow
}
