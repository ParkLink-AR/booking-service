package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// Booking represents a reservation of a parking spot for a time range.
type Booking struct {
	ID            string
	ParkingSpotID string
	UserID        string
	Start         time.Time
	End           time.Time
	Status        enum.BookingStatus
	HoldExpiresAt *time.Time
	// Set only when Status is PENDING_OWNER_APPROVAL: how long the owner has
	// to approve/reject before the sweep auto-cancels and refunds it.
	OwnerResponseDeadline *time.Time
	Amount                int
	PaymentID             string
	// VerificationCode is a 4-digit PIN generated the moment the booking reaches
	// CONFIRMED (see entity.GenerateVerificationCode, called from
	// ConfirmBookingUseCase/ApproveBookingUseCase). The driver reads it in the app and
	// tells it to the owner in person; the owner enters it to transition the booking to
	// IN_PROGRESS (see usecase.VerifyBookingCodeUseCase). Nil before CONFIRMED.
	VerificationCode *string
	// VerificationAttempts counts consecutive wrong codes entered against this booking.
	// Verification is refused once it reaches maxVerificationAttempts, regardless of
	// whether the next guess would be correct.
	VerificationAttempts int
	// VerifiedAt is set once, when the code is successfully verified. It survives the
	// later CONFIRMED/IN_PROGRESS -> COMPLETED transition, so it remains the only record
	// of whether a completed booking was ever verified in person.
	VerifiedAt *time.Time
	// Capacity is the number of identical places the spot rents against this same id
	// (a "playa"): up to Capacity active bookings may overlap. Caller-supplied, like
	// ParkingSpotSchedule -- booking-service does not fetch it. 0 is treated as 1.
	Capacity int
	// Kind distinguishes an hourly slot booking from a day/week/month "estadía" -- see
	// CreateBookingInput below for how each is validated and priced differently.
	Kind      enum.BookingKind
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateBookingInput carries the data needed to create a new booking, threaded from the
// controller through the usecase into the service that owns the creation business logic.
// ParkingSpotSchedule is the spot's published weekly schedule as the caller knows it --
// booking-service validates the requested range against it but never fetches it itself.
type CreateBookingInput struct {
	ParkingSpotID       string
	ParkingSpotSchedule []ScheduleEntry
	UserID              string
	Start               time.Time
	End                 time.Time
	// Amount is only trusted as-is for an HOURLY booking. For a STAY booking the service
	// ignores it and recomputes Start/End/Amount itself from parking-service's stay quote
	// (see CreateBooking) -- never from what the caller sent, so a modified client can't
	// pay a day's price for a month's stay.
	Amount int
	// Capacity of the spot as the caller knows it (see Booking.Capacity). 0 == 1.
	Capacity int
	// Kind selects which validation path CreateBooking takes. Zero value (BookingKindHourly)
	// keeps every existing caller's behaviour unchanged.
	Kind enum.BookingKind
	// StayEndDate is required, and only meaningful, when Kind is BookingKindStay -- the
	// calendar date (YYYY-MM-DD) the stay ends, e.g. "2026-09-23" for a stay starting on
	// Start's calendar date and running 9 days. Ignored for an hourly booking.
	StayEndDate string
}
