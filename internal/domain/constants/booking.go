package constants

const (
	// DefaultHoldMinutes is the fallback for BOOKING_HOLD_MINUTES: how long a newly created
	// PENDING_PAYMENT booking holds the time slot before its hold is considered expired.
	DefaultHoldMinutes = 15

	// BuenosAiresTimezone is the IANA timezone every booking timestamp is rendered in.
	BuenosAiresTimezone = "America/Argentina/Buenos_Aires"

	// BookingsTableName is the PostgreSQL table backing the booking repository.
	BookingsTableName = "bookings"

	// ReviewsTableName is the PostgreSQL table backing the review repository.
	ReviewsTableName = "reviews"
)
