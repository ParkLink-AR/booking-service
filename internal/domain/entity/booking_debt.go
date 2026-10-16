package entity

import "time"

// The states a BookingDebt goes through -- see migrations/028_add_booking_debts.sql.
const (
	// DebtPending: nobody has asked the driver for it yet; the next booking they make takes it on.
	DebtPending = "PENDING"
	// DebtReserved: attached to a booking still waiting for its payment.
	DebtReserved = "RESERVED"
	// DebtPaid: the payment of the booking it was attached to was approved.
	DebtPaid = "PAID"
)

// BookingDebt is what a driver owes for staying past the end of a booking (see ComputeOverstay),
// charged on top of their next reservation instead of blocking them. One per booking.
type BookingDebt struct {
	ID     string
	UserID string
	// SourceBookingID is the booking they overstayed.
	SourceBookingID string
	// ChargedBookingID is the booking whose payment includes this debt; nil while it is PENDING.
	ChargedBookingID *string
	// Minutes is how long past the end they stayed, all of which is charged.
	Minutes int
	// Amount is what they owe, centavos.
	Amount int
	Status string
	// CreatedAt/PaidAt: when it was generated and when a payment settled it (nil until then).
	CreatedAt time.Time
	PaidAt    *time.Time
	// The overstayed booking, as the driver needs to recognise it ("reserva del 12/10 en tal
	// cochera"). Filled by the listings; SpotTitle comes from parking-service, not the database.
	SourceStart   time.Time
	SourceEnd     time.Time
	ParkingSpotID string
	SpotTitle     string
}

// DebtSummary is a driver's debts: every one they have (newest first) and how much a new
// booking would add to its price right now.
type DebtSummary struct {
	// PendingAmount is the total of the PENDING ones, centavos.
	PendingAmount int
	Debts         []BookingDebt
}
