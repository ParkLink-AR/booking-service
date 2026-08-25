package entity

// ScheduleEntry is one published window in a parking spot's weekly recurring schedule,
// as provided by the caller of CreateBooking -- booking-service does not fetch this
// itself, it only validates a requested time range against what it's given.
// DayOfWeek: 0=Monday...6=Sunday, mirroring parking-service's convention.
type ScheduleEntry struct {
	DayOfWeek int
	StartHour int
	EndHour   int
}
