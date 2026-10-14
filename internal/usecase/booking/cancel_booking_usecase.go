package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/cancel_booking_repository_mock.go -package=mocks -source=./cancel_booking_usecase.go

// The driver's cancellation policy for a booking the owner already accepted: up to
// fullRefundCutoff before it starts they get everything back; after that, until it starts,
// lateCancellationRefundPercent of it (the owner is paid the rest, see
// ListEligibleBookingsWithoutPayout). Once it started it can't be cancelled. A request the
// owner hasn't answered yet can always be cancelled before it starts, in full.
const (
	fullRefundCutoff              = time.Hour
	lateCancellationRefundPercent = 50
)

// BookingCanceller reads a booking and transitions it CONFIRMED/PENDING_OWNER_APPROVAL -> CANCELLED.
// Implemented by internal/repository/booking.
type BookingCanceller interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	CancelBooking(ctx context.Context, bookingID, reason string, refundedAmount int) (entity.Booking, error)
	CancelExtensions(ctx context.Context, parentID, reason string) ([]entity.Booking, error)
}

// CancellationRefunder gives a driver their money back, all of it or part.
// Implemented by internal/repository/paymentclient.
type CancellationRefunder interface {
	RefundPayment(ctx context.Context, paymentID string) error
	RefundPaymentAmount(ctx context.Context, paymentID string, amountCentavos int) error
}

type CancelBookingUseCase struct {
	canceller BookingCanceller
	refunder  CancellationRefunder
	spots     SpotDetailsLookup
	events    EventPublisher
	now       func() time.Time // injectable for testing
}

func NewCancelBookingUseCase(canceller BookingCanceller, refunder CancellationRefunder, spots SpotDetailsLookup, events EventPublisher) *CancelBookingUseCase {
	return &CancelBookingUseCase{canceller: canceller, refunder: refunder, spots: spots, events: events, now: time.Now}
}

// LateCancellationRefund is what goes back to the driver of a booking of amount (centavos)
// cancelled in its last hour.
func LateCancellationRefund(amount int) int {
	return amount * lateCancellationRefundPercent / 100
}

// Execute cancels bookingID on behalf of callerID (who must be the booking's driver),
// refunds what the policy above gives back, and publishes booking.cancelled so the owner
// is told. Whatever extends the booking is called off with it, refunded in full.
func (uc *CancelBookingUseCase) Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error) {
	booking, err := uc.canceller.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	if booking.UserID != callerID {
		return entity.Booking{}, errors.NewForbidden("no sos el titular de esta reserva")
	}

	now := uc.now()
	if !now.Before(booking.Start) {
		return entity.Booking{}, errors.NewInvalidInput(
			"CANCELLATION_WINDOW_EXPIRED",
			"la reserva ya empezó, así que no se puede cancelar",
		)
	}
	reason, refundedAmount := constants.CancellationReasonDriverCancelled, 0
	late := booking.Status == enum.BookingStatusConfirmed && now.After(booking.Start.Add(-fullRefundCutoff))
	if late {
		reason, refundedAmount = constants.CancellationReasonDriverLateCancelled, LateCancellationRefund(booking.Amount)
	}

	cancelled, err := uc.canceller.CancelBooking(ctx, bookingID, reason, refundedAmount)
	if err != nil {
		return entity.Booking{}, err
	}
	cancelled.RefundedAmount = refundedAmount
	// Tells the owner the slot is free again. The spot (its owner and title) is only needed
	// for that notice, so a failed lookup never fails the cancellation itself.
	if spot, err := uc.spots.GetSpotDetails(ctx, cancelled.ParkingSpotID); err == nil {
		emitBookingEvent(ctx, uc.events, RoutingKeyBookingCancelled,
			newBookingCancelledEvent(cancelled, spot, CancelledByDriver))
	}

	// The booking is already CANCELLED at this point even if the refund call
	// below fails -- a failure here needs manual follow-up. Surfaced to the
	// caller as an upstream error rather than silently swallowed.
	if late {
		err = uc.refunder.RefundPaymentAmount(ctx, cancelled.PaymentID, refundedAmount)
	} else {
		err = uc.refunder.RefundPayment(ctx, cancelled.PaymentID)
	}
	if err != nil {
		return entity.Booking{}, err
	}

	// An extension starts later than the booking it continues, so it's always given back in full.
	if extensions, err := uc.canceller.CancelExtensions(ctx, cancelled.ID, constants.CancellationReasonDriverCancelled); err == nil {
		for _, extension := range extensions {
			_ = uc.refunder.RefundPayment(ctx, extension.PaymentID)
		}
	}

	return cancelled, nil
}
