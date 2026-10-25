package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/approve_booking_repository_mock.go -package=mocks -source=./approve_booking_usecase.go

// BookingApprover reads a booking and transitions it PENDING_OWNER_APPROVAL -> CONFIRMED.
// Implemented by internal/repository/booking.
type BookingApprover interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	ApproveBooking(ctx context.Context, bookingID, verificationCode string) (entity.Booking, error)
}

// SpotOwnerLookup answers who owns a parking spot, to verify the caller acting on a
// booking actually owns its spot. Implemented by internal/repository/parkingclient.
// Used by the reject / detailed-listing / review flows.
type SpotOwnerLookup interface {
	GetSpotOwnerID(ctx context.Context, spotID string) (string, error)
}

// SpotDetailsLookup resolves the owner id AND the title of a booking's parking spot in a
// single call. The approve flow needs the owner to authorize the caller, and the title to
// populate the booking.confirmed event it publishes. Implemented by
// internal/repository/parkingclient.
type SpotDetailsLookup interface {
	GetSpotDetails(ctx context.Context, spotID string) (entity.SpotDetails, error)
}

type ApproveBookingUseCase struct {
	approver BookingApprover
	spots    SpotDetailsLookup
	events   EventPublisher
	now      func() time.Time // injectable for testing
}

func NewApproveBookingUseCase(approver BookingApprover, spots SpotDetailsLookup, events EventPublisher) *ApproveBookingUseCase {
	return &ApproveBookingUseCase{approver: approver, spots: spots, events: events, now: time.Now}
}

// WithClock makes the use case read the time from now -- for tests.
func (uc *ApproveBookingUseCase) WithClock(now func() time.Time) *ApproveBookingUseCase {
	uc.now = now
	return uc
}

// Execute approves bookingID on behalf of callerID, who must own the booking's parking
// spot, and publishes booking.confirmed so the driver is notified.
func (uc *ApproveBookingUseCase) Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error) {
	booking, err := uc.approver.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, err
	}
	if spot.OwnerID != callerID {
		return entity.Booking{}, errors.NewForbidden("no sos el dueño de esta cochera")
	}
	// Too late to accept: its time to answer ran out (the sweep cancels and refunds it), or the
	// booking is already over.
	now := uc.now()
	if booking.OwnerResponseDeadline != nil && !now.Before(*booking.OwnerResponseDeadline) {
		return entity.Booking{}, errors.NewInvalidInput("APPROVAL_EXPIRED",
			"venció el plazo para aceptar esta reserva: se cancela y se le devuelve el dinero al conductor")
	}
	if !now.Before(booking.End) {
		return entity.Booking{}, errors.NewInvalidInput("APPROVAL_EXPIRED", "esta reserva ya terminó")
	}

	code, err := entity.GenerateVerificationCode()
	if err != nil {
		return entity.Booking{}, err
	}
	approved, err := uc.approver.ApproveBooking(ctx, bookingID, code)
	if err != nil {
		return entity.Booking{}, err
	}

	emitBookingEvent(ctx, uc.events, RoutingKeyBookingConfirmed,
		newBookingEvent(RoutingKeyBookingConfirmed, approved, spot))
	return approved, nil
}
