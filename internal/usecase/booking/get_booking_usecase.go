package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_booking_repository_mock.go -package=mocks -source=./get_booking_usecase.go

// BookingGetter reads a single booking by id. Implemented by internal/repository/booking.
type BookingGetter interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
}

// GetBookingUseCase powers GET /bookings/{id} -- the driver polls this after paying to
// find out whether their booking confirmed outright or is waiting on the owner, and both
// the driver and the spot's owner open it from the app.
type GetBookingUseCase struct {
	getter BookingGetter
	owners SpotOwnerLookup
}

func NewGetBookingUseCase(getter BookingGetter, owners SpotOwnerLookup) *GetBookingUseCase {
	return &GetBookingUseCase{getter: getter, owners: owners}
}

// Execute returns bookingID if viewer is privileged, is its driver, or owns its spot;
// otherwise a Forbidden error.
func (uc *GetBookingUseCase) Execute(ctx context.Context, bookingID string, viewer entity.BookingViewer) (entity.Booking, error) {
	booking, err := uc.getter.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}
	if viewer.Privileged || (viewer.UserID != "" && viewer.UserID == booking.UserID) {
		return booking, nil
	}
	if viewer.UserID != "" {
		ownerID, err := uc.owners.GetSpotOwnerID(ctx, booking.ParkingSpotID)
		if err != nil {
			return entity.Booking{}, err
		}
		if ownerID == viewer.UserID {
			return booking, nil
		}
	}
	return entity.Booking{}, errors.NewForbidden("no sos parte de esta reserva")
}
