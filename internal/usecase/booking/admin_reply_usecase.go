package booking

import (
	"context"
	"strings"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/admin_reply_repository_mock.go -package=mocks -source=./admin_reply_usecase.go

// TicketStatusUpdater sets a support ticket's status. Implemented by internal/repository/booking.
type TicketStatusUpdater interface {
	UpdateTicketStatus(ctx context.Context, ticketID string, status enum.TicketStatus) (entity.SupportTicket, error)
}

// AdminReplyUseCase powers POST /admin/tickets/{id}/messages. A reply from ParkLink staff
// moves a freshly OPENed ticket to IN_PROGRESS automatically (any other status is left as the
// admin set it, e.g. replying on a RESOLVED ticket doesn't silently reopen it) and notifies the
// ticket's creator via push.
type AdminReplyUseCase struct {
	tickets  TicketGetter
	messages TicketMessageCreator
	status   TicketStatusUpdater
	events   EventPublisher
}

func NewAdminReplyUseCase(tickets TicketGetter, messages TicketMessageCreator, status TicketStatusUpdater, events EventPublisher) *AdminReplyUseCase {
	return &AdminReplyUseCase{tickets: tickets, messages: messages, status: status, events: events}
}

func (uc *AdminReplyUseCase) Execute(ctx context.Context, ticketID, adminUserID, body string) (entity.SupportTicketMessage, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return entity.SupportTicketMessage{}, errors.NewInvalidInput("EMPTY_MESSAGE", "el mensaje no puede estar vacío")
	}
	if len(trimmed) > maxMessageBodyLength {
		return entity.SupportTicketMessage{}, errors.NewInvalidInput("MESSAGE_TOO_LONG", "el mensaje es demasiado largo")
	}

	ticket, err := uc.tickets.GetTicketByID(ctx, ticketID)
	if err != nil {
		return entity.SupportTicketMessage{}, err
	}

	created, err := uc.messages.CreateTicketMessage(ctx, entity.SupportTicketMessage{
		TicketID: ticketID,
		SenderID: adminUserID,
		IsAdmin:  true,
		Body:     trimmed,
	})
	if err != nil {
		return entity.SupportTicketMessage{}, err
	}

	if ticket.Status == enum.TicketStatusOpen {
		if updated, err := uc.status.UpdateTicketStatus(ctx, ticketID, enum.TicketStatusInProgress); err == nil {
			ticket = updated
		}
	}

	emitBookingEvent(ctx, uc.events, RoutingKeySupportTicketReplied, newSupportTicketRepliedEvent(created, ticket))

	return created, nil
}
