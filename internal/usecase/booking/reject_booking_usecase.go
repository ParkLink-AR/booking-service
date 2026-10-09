package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/reject_booking_repository_mock.go -package=mocks -source=./reject_booking_usecase.go

// BookingRejecter reads a booking and transitions it PENDING_OWNER_APPROVAL -> CANCELLED.
// Implemented by internal/repository/booking.
type BookingRejecter interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	RejectBooking(ctx context.Context, bookingID string) (entity.Booking, error)
}

// PaymentRefunder issues a refund for an already-charged payment. Implemented by
// internal/repository/paymentclient.
type PaymentRefunder interface {
	RefundPayment(ctx context.Context, paymentID string) error
}

type RejectBookingUseCase struct {
	rejecter BookingRejecter
	spots    SpotDetailsLookup
	refunder PaymentRefunder
	events   EventPublisher
}

func NewRejectBookingUseCase(rejecter BookingRejecter, spots SpotDetailsLookup, refunder PaymentRefunder, events EventPublisher) *RejectBookingUseCase {
	return &RejectBookingUseCase{rejecter: rejecter, spots: spots, refunder: refunder, events: events}
}

// Execute rejects bookingID on behalf of callerID (who must own the booking's parking
// spot), refunds the payment that was already charged for it, and publishes
// booking.cancelled so the driver is told.
func (uc *RejectBookingUseCase) Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error) {
	booking, err := uc.rejecter.GetByID(ctx, bookingID)
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

	rejected, err := uc.rejecter.RejectBooking(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}
	// Announced as soon as it's cancelled: the driver must learn their booking is off even
	// if the refund below needs a manual retry.
	emitBookingEvent(ctx, uc.events, RoutingKeyBookingCancelled,
		newBookingCancelledEvent(rejected, spot, CancelledByOwnerRejected))

	// The booking is already CANCELLED at this point. A refund Mercado Pago fails is not
	// an error here: payment-service keeps it on record as owed (its retry job, and the
	// admin panel's "Reembolsos pendientes"). Only a refund that couldn't even be put on
	// record (payment-service unreachable, no transaction to flag) is surfaced to the
	// caller -- retrying the reject won't work (no longer PENDING_OWNER_APPROVAL), so it
	// needs manual follow-up.
	if err := uc.refunder.RefundPayment(ctx, rejected.PaymentID); err != nil {
		return entity.Booking{}, err
	}

	return rejected, nil
}
