package entity

// AccountClosureSummary is what closing (or suspending) an account would do in the
// bookings domain -- see usecase.AccountClosureUseCase.
type AccountClosureSummary struct {
	// Bookings happening right now, as the driver or on one of the owner's spots. Any of
	// them blocks the closure until it ends.
	InProgressCount int
	// Bookings that haven't started yet and would be cancelled with a full refund.
	DriverBookingsToCancel int
	OwnerBookingsToCancel  int
	// Requests on the owner's spots still waiting for an answer, which would be rejected.
	OwnerRequestsToReject int
	// Money ParkLink still owes the owner (centavos) -- the account's personal data can't
	// be wiped until it's paid.
	PendingPayoutCount  int
	PendingPayoutAmount int
}

// AccountClosureResult is what AccountClosureUseCase.CancelBookings actually did.
type AccountClosureResult struct {
	Cancelled    int
	ExpiredHolds int
	// Bookings that were cancelled but whose refund failed -- they need manual follow-up.
	RefundFailures []string
}
