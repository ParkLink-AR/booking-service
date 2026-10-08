package booking

import (
	"context"
	"strings"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/send_message_repository_mock.go -package=mocks -source=./send_message_usecase.go

// maxMessageBodyLength keeps a message to roughly a couple of paragraphs -- this is a
// quick coordination channel (access instructions, "estoy llegando"), not a full chat client.
const maxMessageBodyLength = 2000

// MessageCreator inserts a booking message. Implemented by internal/repository/booking.
type MessageCreator interface {
	CreateMessage(ctx context.Context, message entity.BookingMessage) (entity.BookingMessage, error)
}

// SendMessageUseCase lets either side of a booking (the driver or the spot's owner) send
// a message to the other -- e.g. access instructions before the driver arrives. Unlike
// reviews, there's no status restriction: a message can be sent as soon as the booking
// exists, all the way through COMPLETED, since coordination is often needed right up to
// (and sometimes after) the visit.
type SendMessageUseCase struct {
	getter  BookingGetter
	spots   SpotDetailsLookup
	creator MessageCreator
	events  EventPublisher
}

func NewSendMessageUseCase(getter BookingGetter, spots SpotDetailsLookup, creator MessageCreator, events EventPublisher) *SendMessageUseCase {
	return &SendMessageUseCase{getter: getter, spots: spots, creator: creator, events: events}
}

func (uc *SendMessageUseCase) Execute(ctx context.Context, bookingID, callerID, body string) (entity.BookingMessage, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return entity.BookingMessage{}, errors.NewInvalidInput("EMPTY_MESSAGE", "el mensaje no puede estar vacío")
	}
	if len(trimmed) > maxMessageBodyLength {
		return entity.BookingMessage{}, errors.NewInvalidInput("MESSAGE_TOO_LONG", "el mensaje es demasiado largo")
	}

	booking, err := uc.getter.GetByID(ctx, bookingID)
	if err != nil {
		return entity.BookingMessage{}, err
	}

	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.BookingMessage{}, err
	}

	var recipientID, recipientRole string
	switch callerID {
	case booking.UserID:
		recipientID, recipientRole = spot.OwnerID, RecipientRoleOwner
	case spot.OwnerID:
		recipientID, recipientRole = booking.UserID, RecipientRoleDriver
	default:
		return entity.BookingMessage{}, errors.NewForbidden("no sos parte de esta reserva")
	}

	created, err := uc.creator.CreateMessage(ctx, entity.BookingMessage{
		BookingID: bookingID,
		SenderID:  callerID,
		Body:      trimmed,
	})
	if err != nil {
		return entity.BookingMessage{}, err
	}

	emitBookingEvent(ctx, uc.events, RoutingKeyBookingMessageSent,
		newBookingMessageEvent(created, spot.Title, booking.ParkingSpotID, recipientID, recipientRole))

	return created, nil
}
