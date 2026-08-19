package constants

const (
	// DefaultMaxAvailabilityDays is the fallback for BOOKING_AVAILABILITY_MAX_DAYS.
	DefaultMaxAvailabilityDays = 14

	// BuenosAiresTimezone is the IANA timezone every availability calculation and the
	// published-base mock operate in.
	BuenosAiresTimezone = "America/Argentina/Buenos_Aires"

	// BookingsTableName is the PostgreSQL table backing the booking repository.
	BookingsTableName = "bookings"

	// MockPublishedOpenHour and MockPublishedCloseHour are the fixed local hours the
	// parking-service mock publishes as available on weekdays (Mon-Fri).
	MockPublishedOpenHour  = 8
	MockPublishedCloseHour = 20
)
