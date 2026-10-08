package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/cancel_booking_repository_mock.go -package=mocks -source=./cancel_booking_usecase.go

// cancellationCutoff is how long before the booking's start time the client must cancel.
// After this window has elapsed, cancellations are rejected.
const cancellationCutoff = time.Hour

// BookingCanceller reads a booking and transitions it CONFIRMED/PENDING_OWNER_APPROVAL -> CANCELLED.
// Implemented by internal/repository/booking.
type BookingCanceller interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	CancelBooking(ctx context.Context, bookingID string) (entity.Booking, error)
}

type CancelBookingUseCase struct {
	canceller BookingCanceller
	refunder  PaymentRefunder
	spots     SpotDetailsLookup
	events    EventPublisher
	now       func() time.Time // injectable for testing
}

func NewCancelBookingUseCase(canceller BookingCanceller, refunder PaymentRefunder, spots SpotDetailsLookup, events EventPublisher) *CancelBookingUseCase {
	return &CancelBookingUseCase{canceller: canceller, refunder: refunder, spots: spots, events: events, now: time.Now}
}

// Execute cancels bookingID on behalf of callerID (who must be the booking's driver),
// refunds the payment that was already charged, and publishes booking.cancelled so the
// owner is told. Cancellation is rejected when the current
// time is less than cancellationCutoff (1 hour) before the booking's start.
func (uc *CancelBookingUseCase) Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error) {
	booking, err := uc.canceller.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	if booking.UserID != callerID {
		return entity.Booking{}, errors.NewForbidden("no sos el titular de esta reserva")
	}

	deadline := booking.Start.Add(-cancellationCutoff)
	if uc.now().After(deadline) {
		return entity.Booking{}, errors.NewInvalidInput(
			"CANCELLATION_WINDOW_EXPIRED",
			"no es posible cancelar con menos de 1 hora de anticipación",
		)
	}

	cancelled, err := uc.canceller.CancelBooking(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}
	// Tells the owner the slot is free again. The spot (its owner and title) is only needed
	// for that notice, so a failed lookup never fails the cancellation itself.
	if spot, err := uc.spots.GetSpotDetails(ctx, cancelled.ParkingSpotID); err == nil {
		emitBookingEvent(ctx, uc.events, RoutingKeyBookingCancelled,
			newBookingCancelledEvent(cancelled, spot, CancelledByDriver))
	}

	// The booking is already CANCELLED at this point even if the refund call
	// below fails -- a failure here needs manual follow-up. Surfaced to the
	// caller as an upstream error rather than silently swallowed.
	if err := uc.refunder.RefundPayment(ctx, cancelled.PaymentID); err != nil {
		return entity.Booking{}, err
	}

	return cancelled, nil
}
