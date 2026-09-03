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
	owners   SpotOwnerLookup
	refunder PaymentRefunder
}

func NewRejectBookingUseCase(rejecter BookingRejecter, owners SpotOwnerLookup, refunder PaymentRefunder) *RejectBookingUseCase {
	return &RejectBookingUseCase{rejecter: rejecter, owners: owners, refunder: refunder}
}

// Execute rejects bookingID on behalf of callerID (who must own the booking's parking
// spot) and refunds the payment that was already charged for it.
func (uc *RejectBookingUseCase) Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error) {
	booking, err := uc.rejecter.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	ownerID, err := uc.owners.GetSpotOwnerID(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, err
	}
	if ownerID != callerID {
		return entity.Booking{}, errors.NewForbidden("no sos el dueño de esta cochera")
	}

	rejected, err := uc.rejecter.RejectBooking(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	// The booking is already CANCELLED at this point even if the refund call
	// below fails -- retrying by calling reject again won't work (status is
	// no longer PENDING_OWNER_APPROVAL), so a failure here needs manual
	// follow-up. Surfaced to the caller as an upstream error rather than
	// silently swallowed.
	if err := uc.refunder.RefundPayment(ctx, rejected.PaymentID); err != nil {
		return entity.Booking{}, err
	}

	return rejected, nil
}
