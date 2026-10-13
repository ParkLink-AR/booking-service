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
// a message to the other -- e.g. access instructions before the driver arrives. Only once
// the booking has been paid: an unpaid one (still awaiting payment, or expired without it)
// isn't a real booking yet, and mustn't open a channel to the owner. From then on there's
// no status restriction, all the way through COMPLETED (or CANCELLED), since coordination
// is often needed right up to (and sometimes after) the visit.
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

	if !bookingWasPaid(booking) {
		return entity.BookingMessage{}, errors.NewInvalidInput("BOOKING_NOT_PAID",
			"la reserva todavía no está pagada: los mensajes se habilitan cuando se acredita el pago")
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

// bookingWasPaid reports whether the booking's payment ever went through. PaidAt is the
// record of that, but bookings from before it existed only have their PaymentID (and a
// cash one is "CASH") -- either means the money moved.
func bookingWasPaid(booking entity.Booking) bool {
	return booking.PaidAt != nil || booking.PaymentID != ""
}
