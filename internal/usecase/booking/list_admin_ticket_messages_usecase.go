package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

// ListAdminTicketMessagesUseCase powers GET /admin/tickets/{id}/messages -- the admin-facing
// equivalent of ListTicketMessagesUseCase, minus the ownership check (the gateway's admin gate
// already covers authorization for every /admin/* route).
type ListAdminTicketMessagesUseCase struct {
	tickets  TicketGetter
	messages TicketMessageLister
}

func NewListAdminTicketMessagesUseCase(tickets TicketGetter, messages TicketMessageLister) *ListAdminTicketMessagesUseCase {
	return &ListAdminTicketMessagesUseCase{tickets: tickets, messages: messages}
}

func (uc *ListAdminTicketMessagesUseCase) Execute(ctx context.Context, ticketID string) ([]entity.SupportTicketMessage, error) {
	if _, err := uc.tickets.GetTicketByID(ctx, ticketID); err != nil {
		return nil, err
	}
	return uc.messages.ListMessagesByTicket(ctx, ticketID)
}
