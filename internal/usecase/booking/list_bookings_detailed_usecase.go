package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_bookings_detailed_repository_mock.go -package=mocks -source=./list_bookings_detailed_usecase.go

// BookingListerBySpot lists every booking (with id, status, amount) for a parking spot.
// Implemented by internal/repository/booking.
type BookingListerBySpot interface {
	ListBookingsBySpot(ctx context.Context, spotID string) ([]entity.Booking, error)
}

// ListBookingsDetailedUseCase powers the owner's "solicitudes de reserva" screen: the
// full booking rows for one of their spots, not the day-grouped calendar view that
// ListBookingsUseCase returns for public availability checks.
type ListBookingsDetailedUseCase struct {
	lister BookingListerBySpot
	owners SpotOwnerLookup
}

func NewListBookingsDetailedUseCase(lister BookingListerBySpot, owners SpotOwnerLookup) *ListBookingsDetailedUseCase {
	return &ListBookingsDetailedUseCase{lister: lister, owners: owners}
}

func (uc *ListBookingsDetailedUseCase) Execute(ctx context.Context, spotID, callerID string) ([]entity.Booking, error) {
	ownerID, err := uc.owners.GetSpotOwnerID(ctx, spotID)
	if err != nil {
		return nil, err
	}
	if ownerID != callerID {
		return nil, errors.NewForbidden("no sos el dueño de esta cochera")
	}

	return uc.lister.ListBookingsBySpot(ctx, spotID)
}
