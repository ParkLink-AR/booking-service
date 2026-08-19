package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// TimeWindow is a contiguous half-open time interval [Start, End).
type TimeWindow struct {
	Start time.Time
	End   time.Time
}

// DayBase is one day of published availability from parking-service, in local time.
type DayBase struct {
	Date    time.Time
	Windows []TimeWindow
}

// AvailabilityBase is the published availability for a spot across a date range.
type AvailabilityBase struct {
	Days []DayBase
}

// DayAvailability is the effective availability for one day, after subtracting active bookings.
type DayAvailability struct {
	Date        time.Time
	Status      enum.AvailabilityStatus
	FreeWindows []TimeWindow
}
