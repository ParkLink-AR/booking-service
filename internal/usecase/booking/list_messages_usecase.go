package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_messages_repository_mock.go -package=mocks -source=./list_messages_usecase.go

// MessageLister reads every message on a booking. Implemented by internal/repository/booking.
type MessageLister interface {
	ListMessagesByBooking(ctx context.Context, bookingID string) ([]entity.BookingMessage, error)
}

// ListMessagesUseCase powers GET /bookings/{id}/messages. Only the booking's driver or
// its spot's owner can read the thread -- it's a private coordination channel between
// exactly those two people, not a public review.
type ListMessagesUseCase struct {
	getter BookingGetter
	owners SpotOwnerLookup
	lister MessageLister
}

func NewListMessagesUseCase(getter BookingGetter, owners SpotOwnerLookup, lister MessageLister) *ListMessagesUseCase {
	return &ListMessagesUseCase{getter: getter, owners: owners, lister: lister}
}

func (uc *ListMessagesUseCase) Execute(ctx context.Context, bookingID, callerID string) ([]entity.BookingMessage, error) {
	booking, err := uc.getter.GetByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	if callerID != booking.UserID {
		ownerID, err := uc.owners.GetSpotOwnerID(ctx, booking.ParkingSpotID)
		if err != nil {
			return nil, err
		}
		if callerID != ownerID {
			return nil, errors.NewForbidden("no sos parte de esta reserva")
		}
	}

	return uc.lister.ListMessagesByBooking(ctx, bookingID)
}
