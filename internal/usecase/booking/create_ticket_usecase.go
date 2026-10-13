package booking

import (
	"context"
	"strings"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/create_ticket_repository_mock.go -package=mocks -source=./create_ticket_usecase.go

// maxTicketSubjectLength keeps the subject to a one-line summary -- the actual issue goes in
// the ticket's opening message.
const maxTicketSubjectLength = 200

// TicketCreator inserts a support ticket. Implemented by internal/repository/booking.
type TicketCreator interface {
	CreateTicket(ctx context.Context, ticket entity.SupportTicket) (entity.SupportTicket, error)
}

// TicketMessageCreator inserts a message on a support ticket's thread. Implemented by
// internal/repository/booking.
type TicketMessageCreator interface {
	CreateTicketMessage(ctx context.Context, message entity.SupportTicketMessage) (entity.SupportTicketMessage, error)
}

// CreateTicketUseCase lets either side of a booking (the driver or the spot's owner) open a
// support ticket against it -- e.g. a payment issue or a damage report. The ticket is created
// together with its opening message in one call, so callers never see a ticket with an empty
// thread.
type CreateTicketUseCase struct {
	getter   BookingGetter
	owners   SpotOwnerLookup
	tickets  TicketCreator
	messages TicketMessageCreator
	events   EventPublisher
}

func NewCreateTicketUseCase(getter BookingGetter, owners SpotOwnerLookup, tickets TicketCreator, messages TicketMessageCreator, events EventPublisher) *CreateTicketUseCase {
	return &CreateTicketUseCase{getter: getter, owners: owners, tickets: tickets, messages: messages, events: events}
}

func (uc *CreateTicketUseCase) Execute(ctx context.Context, bookingID, callerID, subject, body string) (entity.SupportTicket, error) {
	trimmedSubject := strings.TrimSpace(subject)
	if trimmedSubject == "" {
		return entity.SupportTicket{}, errors.NewInvalidInput("EMPTY_SUBJECT", "el asunto no puede estar vacío")
	}
	if len(trimmedSubject) > maxTicketSubjectLength {
		return entity.SupportTicket{}, errors.NewInvalidInput("SUBJECT_TOO_LONG", "el asunto es demasiado largo")
	}
	trimmedBody := strings.TrimSpace(body)
	if trimmedBody == "" {
		return entity.SupportTicket{}, errors.NewInvalidInput("EMPTY_MESSAGE", "el mensaje no puede estar vacío")
	}
	if len(trimmedBody) > maxMessageBodyLength {
		return entity.SupportTicket{}, errors.NewInvalidInput("MESSAGE_TOO_LONG", "el mensaje es demasiado largo")
	}

	booking, err := uc.getter.GetByID(ctx, bookingID)
	if err != nil {
		return entity.SupportTicket{}, err
	}

	if callerID != booking.UserID {
		ownerID, err := uc.owners.GetSpotOwnerID(ctx, booking.ParkingSpotID)
		if err != nil {
			return entity.SupportTicket{}, err
		}
		if callerID != ownerID {
			return entity.SupportTicket{}, errors.NewForbidden("no sos parte de esta reserva")
		}
	}

	ticket, err := uc.tickets.CreateTicket(ctx, entity.SupportTicket{
		BookingID: bookingID,
		CreatedBy: callerID,
		Subject:   trimmedSubject,
	})
	if err != nil {
		return entity.SupportTicket{}, err
	}

	if _, err := uc.messages.CreateTicketMessage(ctx, entity.SupportTicketMessage{
		TicketID: ticket.ID,
		SenderID: callerID,
		IsAdmin:  false,
		Body:     trimmedBody,
	}); err != nil {
		return entity.SupportTicket{}, err
	}

	// ParkLink staff hear about it right away (email to the support inbox).
	authorRole := RecipientRoleOwner
	if callerID == booking.UserID {
		authorRole = RecipientRoleDriver
	}
	emitBookingEvent(ctx, uc.events, RoutingKeySupportTicketOpened,
		newSupportTicketStaffEvent(RoutingKeySupportTicketOpened, ticket, authorRole, trimmedBody))

	return ticket, nil
}
