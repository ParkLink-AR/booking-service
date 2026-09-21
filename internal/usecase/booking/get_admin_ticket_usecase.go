package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_admin_ticket_repository_mock.go -package=mocks -source=./get_admin_ticket_usecase.go

// GetAdminTicketUseCase powers GET /admin/tickets/{id}. It enriches the ticket with its
// booking and parking spot so admin-dashboard can render full context (who, which spot, which
// dates) without a second round trip through the driver/owner-gated booking endpoints.
type GetAdminTicketUseCase struct {
	tickets TicketGetter
	getter  BookingGetter
	spots   SpotDetailsLookup
}

func NewGetAdminTicketUseCase(tickets TicketGetter, getter BookingGetter, spots SpotDetailsLookup) *GetAdminTicketUseCase {
	return &GetAdminTicketUseCase{tickets: tickets, getter: getter, spots: spots}
}

func (uc *GetAdminTicketUseCase) Execute(ctx context.Context, ticketID string) (entity.SupportTicket, entity.Booking, entity.SpotDetails, error) {
	ticket, err := uc.tickets.GetTicketByID(ctx, ticketID)
	if err != nil {
		return entity.SupportTicket{}, entity.Booking{}, entity.SpotDetails{}, err
	}

	booking, err := uc.getter.GetByID(ctx, ticket.BookingID)
	if err != nil {
		return entity.SupportTicket{}, entity.Booking{}, entity.SpotDetails{}, err
	}

	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.SupportTicket{}, entity.Booking{}, entity.SpotDetails{}, err
	}

	return ticket, booking, spot, nil
}
