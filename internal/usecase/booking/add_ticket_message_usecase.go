package booking

import (
	"context"
	"strings"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/add_ticket_message_repository_mock.go -package=mocks -source=./add_ticket_message_usecase.go

// AddTicketMessageUseCase lets a ticket's creator add a follow-up message to their own
// ticket's thread (e.g. answering a question ParkLink staff asked). The admin-facing
// equivalent is AdminReplyUseCase. Writing on a RESOLVED ticket reopens it (the problem
// wasn't solved after all); a CLOSED one is final and takes no more messages -- a new ticket
// is the way. Either way ParkLink staff are told (email to the support inbox).
type AddTicketMessageUseCase struct {
	tickets  TicketGetter
	messages TicketMessageCreator
	status   TicketStatusUpdater
	bookings TicketBookingGetter
	events   EventPublisher
}

func NewAddTicketMessageUseCase(tickets TicketGetter, messages TicketMessageCreator, status TicketStatusUpdater, bookings TicketBookingGetter, events EventPublisher) *AddTicketMessageUseCase {
	return &AddTicketMessageUseCase{tickets: tickets, messages: messages, status: status, bookings: bookings, events: events}
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
	if ticket.Status == enum.TicketStatusClosed {
		return entity.SupportTicketMessage{}, errors.NewInvalidInput("TICKET_CLOSED",
			"este ticket está cerrado: si necesitás ayuda, abrí uno nuevo")
	}

	created, err := uc.messages.CreateTicketMessage(ctx, entity.SupportTicketMessage{
		TicketID: ticketID,
		SenderID: callerID,
		IsAdmin:  false,
		Body:     trimmed,
	})
	if err != nil {
		return entity.SupportTicketMessage{}, err
	}

	if ticket.Status == enum.TicketStatusResolved {
		if updated, err := uc.status.UpdateTicketStatus(ctx, ticketID, enum.TicketStatusOpen); err == nil {
			ticket = updated
		}
	}

	emitBookingEvent(ctx, uc.events, RoutingKeySupportTicketUserMessage,
		newSupportTicketStaffEvent(RoutingKeySupportTicketUserMessage, ticket, uc.authorRole(ctx, ticket), trimmed))

	return created, nil
}

// authorRole is which side of the booking the ticket's creator is -- see
// AdminReplyUseCase.creatorRole. Empty when the booking can't be read.
func (uc *AddTicketMessageUseCase) authorRole(ctx context.Context, ticket entity.SupportTicket) string {
	booking, err := uc.bookings.GetByID(ctx, ticket.BookingID)
	if err != nil {
		return ""
	}
	if booking.UserID == ticket.CreatedBy {
		return RecipientRoleDriver
	}
	return RecipientRoleOwner
}
