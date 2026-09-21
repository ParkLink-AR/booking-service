package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_ticket_messages_repository_mock.go -package=mocks -source=./list_ticket_messages_usecase.go

// TicketMessageLister reads every message on a support ticket's thread. Implemented by
// internal/repository/booking.
type TicketMessageLister interface {
	ListMessagesByTicket(ctx context.Context, ticketID string) ([]entity.SupportTicketMessage, error)
}

// ListTicketMessagesUseCase powers GET /tickets/{id}/messages for the ticket's own creator
// (the admin-facing equivalent is ListAdminTicketMessagesUseCase, which skips this ownership
// check since the caller already passed the gateway's admin gate).
type ListTicketMessagesUseCase struct {
	tickets  TicketGetter
	messages TicketMessageLister
}

func NewListTicketMessagesUseCase(tickets TicketGetter, messages TicketMessageLister) *ListTicketMessagesUseCase {
	return &ListTicketMessagesUseCase{tickets: tickets, messages: messages}
}

func (uc *ListTicketMessagesUseCase) Execute(ctx context.Context, ticketID, callerID string) ([]entity.SupportTicketMessage, error) {
	ticket, err := uc.tickets.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if callerID != ticket.CreatedBy {
		return nil, errors.NewForbidden("no sos el dueño de este ticket")
	}
	return uc.messages.ListMessagesByTicket(ctx, ticketID)
}
