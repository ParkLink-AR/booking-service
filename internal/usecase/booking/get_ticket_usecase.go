package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_ticket_repository_mock.go -package=mocks -source=./get_ticket_usecase.go

// TicketGetter reads a single support ticket by id. Implemented by internal/repository/booking.
type TicketGetter interface {
	GetTicketByID(ctx context.Context, ticketID string) (entity.SupportTicket, error)
}

// GetTicketUseCase powers GET /tickets/{id}. Only the ticket's own creator can read it -- a
// driver can't peek at the owner's tickets on the same booking, or vice versa.
type GetTicketUseCase struct {
	tickets TicketGetter
}

func NewGetTicketUseCase(tickets TicketGetter) *GetTicketUseCase {
	return &GetTicketUseCase{tickets: tickets}
}

func (uc *GetTicketUseCase) Execute(ctx context.Context, ticketID, callerID string) (entity.SupportTicket, error) {
	ticket, err := uc.tickets.GetTicketByID(ctx, ticketID)
	if err != nil {
		return entity.SupportTicket{}, err
	}
	if callerID != ticket.CreatedBy {
		return entity.SupportTicket{}, errors.NewForbidden("no sos el dueño de este ticket")
	}
	return ticket, nil
}
