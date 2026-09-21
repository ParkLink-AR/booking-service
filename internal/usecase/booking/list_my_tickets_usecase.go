package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_my_tickets_repository_mock.go -package=mocks -source=./list_my_tickets_usecase.go

// TicketListerByBookingAndCreator reads every ticket a caller opened on a booking. Implemented
// by internal/repository/booking.
type TicketListerByBookingAndCreator interface {
	ListTicketsByBookingAndCreator(ctx context.Context, bookingID, createdBy string) ([]entity.SupportTicket, error)
}

// ListMyTicketsUseCase powers GET /bookings/{id}/tickets. Only the booking's driver or its
// spot's owner can list tickets on it, and each side only sees the tickets they themselves
// opened -- not the other side's.
type ListMyTicketsUseCase struct {
	getter  BookingGetter
	owners  SpotOwnerLookup
	tickets TicketListerByBookingAndCreator
}

func NewListMyTicketsUseCase(getter BookingGetter, owners SpotOwnerLookup, tickets TicketListerByBookingAndCreator) *ListMyTicketsUseCase {
	return &ListMyTicketsUseCase{getter: getter, owners: owners, tickets: tickets}
}

func (uc *ListMyTicketsUseCase) Execute(ctx context.Context, bookingID, callerID string) ([]entity.SupportTicket, error) {
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

	return uc.tickets.ListTicketsByBookingAndCreator(ctx, bookingID, callerID)
}
