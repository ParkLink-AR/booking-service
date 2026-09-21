package booking

import (
	"context"
	"strings"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/add_ticket_message_repository_mock.go -package=mocks -source=./add_ticket_message_usecase.go

// AddTicketMessageUseCase lets a ticket's creator add a follow-up message to their own
// ticket's thread (e.g. answering a question ParkLink staff asked). The admin-facing
// equivalent is AdminReplyUseCase.
type AddTicketMessageUseCase struct {
	tickets  TicketGetter
	messages TicketMessageCreator
}

func NewAddTicketMessageUseCase(tickets TicketGetter, messages TicketMessageCreator) *AddTicketMessageUseCase {
	return &AddTicketMessageUseCase{tickets: tickets, messages: messages}
}

func (uc *AddTicketMessageUseCase) Execute(ctx context.Context, ticketID, callerID, body string) (entity.SupportTicketMessage, error) {
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
	if callerID != ticket.CreatedBy {
		return entity.SupportTicketMessage{}, errors.NewForbidden("no sos el dueño de este ticket")
	}

	return uc.messages.CreateTicketMessage(ctx, entity.SupportTicketMessage{
		TicketID: ticketID,
		SenderID: callerID,
		IsAdmin:  false,
		Body:     trimmed,
	})
}
