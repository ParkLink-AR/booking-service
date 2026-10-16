package entity

import (
	"math"
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// Who closed a booking's exit.
const (
	ExitClosedByOwnerCode = "OWNER_CODE"
	ExitClosedBySupport   = "SUPPORT"
)

// BookingExit is how a verified hourly booking ends: the driver's second 4-digit code, given
// to the owner as the vehicle leaves, and -- once closed -- when that happened and what staying
// past the end cost. See migrations/027.
type BookingExit struct {
	BookingID string
	Code      string
	Attempts  int
	// ExitedAt is nil until the booking is closed.
	ExitedAt *time.Time
	// ClosedBy is ExitClosedByOwnerCode or ExitClosedBySupport once closed; empty before.
	ClosedBy string
	Overstay Overstay
}

// Closed reports whether the vehicle's departure was recorded.
func (exit BookingExit) Closed() bool { return exit.ExitedAt != nil }

// Overstay is what staying past the booking's end amounts to.
type Overstay struct {
	// Minutes is how long past the end the driver stayed, whether or not it is charged.
	Minutes int
	// ChargedMinutes is Minutes when it went past the grace period, 0 when it didn't.
	ChargedMinutes int
	// Amount is what the driver owes for ChargedMinutes, centavos: their paid rate, commission
	// included.
	Amount int
	// OwnerAmount is what the spot's owner is owed for them, centavos: their own rate, without
	// the commission.
	OwnerAmount int
}

// AwaitingExit reports whether b is a verified hourly booking still waiting for its exit: the
// driver's arrival was confirmed in person, so leaving is confirmed the same way. A booking
// whose arrival was never verified completes at its end as always.
func (b Booking) AwaitingExit() bool {
	if b.Kind != enum.BookingKindHourly || b.VerifiedAt == nil {
		return false
	}
	return b.Status == enum.BookingStatusConfirmed || b.Status == enum.BookingStatusInProgress
}

// ComputeOverstay prices how long past b's end the vehicle stayed (exitedAt). Staying up to the
// grace period costs nothing -- the same 15 minutes that keep the next driver from arriving
// while the spot is still taken -- but once past it, the whole time is charged, from the end.
// It is billed at what the booking itself was billed: its paid amount (commission included)
// and its owner's amount spread over its length, so the owner gets exactly their own rate for
// it and the difference is the platform's, as with the booking.
func ComputeOverstay(b Booking, exitedAt time.Time) Overstay {
	over := exitedAt.Sub(b.End)
	if over <= 0 {
		return Overstay{}
	}
	minutes := int(math.Ceil(over.Minutes()))
	if over <= GracePeriod {
		return Overstay{Minutes: minutes}
	}
	bookedMinutes := b.End.Sub(b.Start).Minutes()
	if bookedMinutes <= 0 {
		return Overstay{Minutes: minutes}
	}
	overstay := Overstay{Minutes: minutes, ChargedMinutes: minutes}
	overstay.Amount = int(math.Round(float64(b.Amount) * float64(minutes) / bookedMinutes))
	if b.OwnerAmount != nil {
		overstay.OwnerAmount = int(math.Round(float64(*b.OwnerAmount) * float64(minutes) / bookedMinutes))
	}
	return overstay
}

// ExitRole is who is looking at a booking's exit.
type ExitRole int

const (
	ExitRoleDriver ExitRole = iota
	ExitRoleOwner
	ExitRoleAdmin
)

// ExitView is a booking's exit as one side of it sees it.
type ExitView struct {
	Role ExitRole
	// Awaiting is true while the vehicle's departure still has to be recorded.
	Awaiting bool
	// Exit is nil until the driver first asks for the code. Its Code is only for the driver.
	Exit *BookingExit
}
