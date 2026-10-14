package constants

const (
	// DefaultHoldMinutes is the fallback for BOOKING_HOLD_MINUTES: how long a newly created
	// PENDING_PAYMENT booking holds the time slot before its hold is considered expired.
	DefaultHoldMinutes = 5

	// GracePeriodMinutes is the buffer a booking keeps the spot occupied after its end (and,
	// symmetrically, that a new booking must leave before the next one starts), in case the
	// driver hasn't left yet. It only affects whether *other* drivers can book -- it is never
	// billed to the driver nor shown to anyone. See entity.OccupancyWindow.
	GracePeriodMinutes = 15

	// BuenosAiresTimezone is the IANA timezone every booking timestamp is rendered in.
	BuenosAiresTimezone = "America/Argentina/Buenos_Aires"

	// BookingsTableName is the PostgreSQL table backing the booking repository.
	BookingsTableName = "bookings"

	// ReviewsTableName is the PostgreSQL table backing the review repository.
	ReviewsTableName = "reviews"

	// ReportsTableName is the PostgreSQL table backing what users report from the app (a
	// spot, a person or a review) for ParkLink staff to look at.
	ReportsTableName = "reports"

	// BookingMessagesTableName is the PostgreSQL table backing in-app messages
	// between a booking's driver and its spot's owner.
	BookingMessagesTableName = "booking_messages"

	// SupportTicketsTableName is the PostgreSQL table backing booking-scoped support tickets.
	SupportTicketsTableName = "support_tickets"

	// SupportTicketMessagesTableName is the PostgreSQL table backing a support ticket's
	// message thread between its creator and ParkLink staff.
	SupportTicketMessagesTableName = "support_ticket_messages"

	// CashPaymentID is the sentinel payment_id a booking gets when the driver
	// chose to pay in person, in cash, instead of through Mercado Pago -- there's
	// no real payment to reference. Must match mobile's CASH_PAYMENT_ID
	// (src/api/booking.ts).
	CashPaymentID = "CASH"

	// CancellationReasonOwnerRejected/OwnerExpired/DriverCancelled are the three ways a
	// booking ends up CANCELLED -- see entity.Booking.CancellationReason and the CHECK
	// constraint in migrations/013_add_cancellation_reason.sql.
	CancellationReasonOwnerRejected   = "OWNER_REJECTED"
	CancellationReasonOwnerExpired    = "OWNER_EXPIRED"
	CancellationReasonDriverCancelled = "DRIVER_CANCELLED"
	// The booking hadn't started yet when the spot owner's (OWNER_ACCOUNT_CLOSED) or the
	// driver's (DRIVER_ACCOUNT_CLOSED) account was closed or suspended -- see
	// usecase.AccountClosureUseCase. The driver always gets a full refund.
	CancellationReasonOwnerAccountClosed  = "OWNER_ACCOUNT_CLOSED"
	CancellationReasonDriverAccountClosed = "DRIVER_ACCOUNT_CLOSED"
	// The owner called off a booking they had already accepted (with a reason, see
	// OwnerCancelBooking): the driver always gets a full refund.
	CancellationReasonOwnerCancelled = "OWNER_CANCELLED"
	// The driver cancelled an accepted booking less than an hour before it started: part of
	// it goes back to them and the owner is paid the rest (see CancelBookingUseCase).
	CancellationReasonDriverLateCancelled = "DRIVER_LATE_CANCELLED"
)
