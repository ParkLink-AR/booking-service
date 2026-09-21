package constants

const (
	// DefaultHoldMinutes is the fallback for BOOKING_HOLD_MINUTES: how long a newly created
	// PENDING_PAYMENT booking holds the time slot before its hold is considered expired.
	DefaultHoldMinutes = 5

	// BuenosAiresTimezone is the IANA timezone every booking timestamp is rendered in.
	BuenosAiresTimezone = "America/Argentina/Buenos_Aires"

	// BookingsTableName is the PostgreSQL table backing the booking repository.
	BookingsTableName = "bookings"

	// ReviewsTableName is the PostgreSQL table backing the review repository.
	ReviewsTableName = "reviews"

	// BookingMessagesTableName is the PostgreSQL table backing in-app messages
	// between a booking's driver and its spot's owner.
	BookingMessagesTableName = "booking_messages"

	// CashPaymentID is the sentinel payment_id a booking gets when the driver
	// chose to pay in person, in cash, instead of through Mercado Pago -- there's
	// no real payment to reference. Must match mobile's CASH_PAYMENT_ID
	// (src/api/booking.ts).
	CashPaymentID = "CASH"
)
