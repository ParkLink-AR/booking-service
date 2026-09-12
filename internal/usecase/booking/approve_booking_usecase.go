package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/approve_booking_repository_mock.go -package=mocks -source=./approve_booking_usecase.go

// BookingApprover reads a booking and transitions it PENDING_OWNER_APPROVAL -> CONFIRMED.
// Implemented by internal/repository/booking.
type BookingApprover interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	ApproveBooking(ctx context.Context, bookingID string) (entity.Booking, error)
}

// SpotOwnerLookup answers who owns a parking spot, to verify the caller approving or
// rejecting a booking actually owns it. Implemented by internal/repository/parkingclient.
type SpotOwnerLookup interface {
	GetSpotOwnerID(ctx context.Context, spotID string) (string, error)
}

type ApproveBookingUseCase struct {
	approver BookingApprover
	owners   SpotOwnerLookup
}

func NewApproveBookingUseCase(approver BookingApprover, owners SpotOwnerLookup) *ApproveBookingUseCase {
	return &ApproveBookingUseCase{approver: approver, owners: owners}
}

// Execute approves bookingID on behalf of callerID, who must own the booking's parking spot.
func (uc *ApproveBookingUseCase) Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error) {
	booking, err := uc.approver.GetByID(ctx, bookingID)
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

	return uc.approver.ApproveBooking(ctx, bookingID)
}
