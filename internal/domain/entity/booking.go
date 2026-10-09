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
	// OwnerAmount is the owner's own price at the moment this booking was priced (see
	// Service.priceHourly/priceStay) -- Amount minus whatever platform commission was
	// added on top for the driver, fixed at creation time so a later admin change to the
	// commission rate never retroactively changes an already-made booking's payout (see
	// GeneratePendingPayoutsUseCase). Nil for a booking that predates this field.
	OwnerAmount *int
	PaymentID   string
	// PaidAt is set once, the moment ConfirmBooking records a successful Mercado Pago
	// payment (PENDING_PAYMENT -> PENDING_OWNER_APPROVAL). It never changes afterward --
	// approve/reject/cancel/verify all leave it untouched -- so it stays the one
	// trustworthy record of whether and when the money actually moved, independent of
	// UpdatedAt (which every one of those transitions bumps). Nil before payment.
	PaidAt *time.Time
	// CancellationReason distinguishes *why* a CANCELLED booking got there --
	// "OWNER_REJECTED" (RejectBooking), "OWNER_EXPIRED" (owner never responded in time,
	// see ExpireStalePendingApprovals), or "DRIVER_CANCELLED" (the driver cancelled it
	// themselves, see CancelBooking). Nil for any non-cancelled booking, and for a
	// cancelled one predating this field. Without it, every CANCELLED booking looked
	// identical from the driver's own cancellation vs. the owner's rejection.
	CancellationReason *string
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
	// VerificationReview is set only on a paid booking that reached COMPLETED without ever
	// being verified (VerifiedAt nil) -- see enum.VerificationReview. While it's PENDING the
	// owner's payout is held back until a ParkLink admin approves or refunds it. Nil for
	// every verified booking and for anything completed before this field existed.
	VerificationReview *enum.VerificationReview
	// Capacity is the number of identical places the spot rents against this same id
	// (a "playa"): up to Capacity active bookings may overlap. Caller-supplied, like
	// ParkingSpotSchedule -- booking-service does not fetch it. 0 is treated as 1.
	Capacity int
	// Kind distinguishes an hourly slot booking from a day/week/month "estadía" -- see
	// CreateBookingInput below for how each is validated and priced differently.
	Kind      enum.BookingKind
	CreatedAt time.Time
	UpdatedAt time.Time
	// VehicleType/Plate/Brand/Model/Country are a snapshot of whichever vehicle (if any)
	// the driver picked from their own saved list (see auth-service's
	// /users/{id}/vehicles) at the moment this booking was created -- copied in, not a
	// live reference, so later editing or deleting that saved vehicle never changes an
	// already-made booking's record. All nil when no vehicle was picked. A bicycle has a
	// type but no plate (and maybe no brand/model); a booking made before vehicles had a
	// type has a plate but no type.
	VehicleType    *string
	VehiclePlate   *string
	VehicleBrand   *string
	VehicleModel   *string
	VehicleCountry *string
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
	// VehicleType/Plate/Brand/Model/Country describe the saved vehicle the driver picked --
	// all empty when they didn't pick one. Whether one is required, and which types fit,
	// depends on the spot (see SpotVehicleRules); a bicycle has no plate.
	VehicleType    string
	VehiclePlate   string
	VehicleBrand   string
	VehicleModel   string
	VehicleCountry string
}

// BookingViewer is who's asking to read a booking. Privileged callers (a ParkLink admin)
// may read any booking; everyone else only one they're part of (its driver, or the owner
// of its spot).
type BookingViewer struct {
	UserID     string
	Privileged bool
}
