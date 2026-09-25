package booking

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/event_publisher_mock.go -package=mocks -source=./events.go

// Routing keys for the booking domain events published to the shared
// `parklink.events` topic exchange. notifications-service binds its own queue
// to these (see notifications-service/app/messaging/consumer.py).
const (
	RoutingKeyBookingPendingApproval = "booking.pending_approval"
	RoutingKeyBookingConfirmed       = "booking.confirmed"
	RoutingKeyBookingVerified        = "booking.verified"
	RoutingKeyBookingMessageSent     = "booking.message_sent"
	RoutingKeySupportTicketReplied   = "support.ticket_replied"
	RoutingKeyPayoutPaid             = "payout.paid"
)

// EventPublisher publishes a domain event to the shared event bus under a routing key.
// Implemented by internal/repository/messaging. Its implementation is responsible for
// logging publish failures; the use cases treat a failed publish as non-fatal.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, payload any) error
}

// bookingEvent is the wire shape of every booking.* domain event. It is deliberately
// self-contained: it carries both party ids AND the spot title, so notifications-service
// (or any other consumer) can build a notification without a follow-up HTTP lookup.
type bookingEvent struct {
	EventID               string     `json:"event_id"`
	EventType             string     `json:"event_type"`
	OccurredAt            time.Time  `json:"occurred_at"`
	BookingID             string     `json:"booking_id"`
	ParkingSpotID         string     `json:"parking_spot_id"`
	ParkingSpotTitle      string     `json:"parking_spot_title"`
	OwnerUserID           string     `json:"owner_user_id"`
	DriverUserID          string     `json:"driver_user_id"`
	Start                 time.Time  `json:"start"`
	End                   time.Time  `json:"end"`
	Amount                int        `json:"amount"`
	PaymentID             string     `json:"payment_id"`
	OwnerResponseDeadline *time.Time `json:"owner_response_deadline,omitempty"`
}

func newBookingEvent(eventType string, b entity.Booking, spot entity.SpotDetails) bookingEvent {
	return bookingEvent{
		EventID:               uuid.NewString(),
		EventType:             eventType,
		OccurredAt:            time.Now().UTC(),
		BookingID:             b.ID,
		ParkingSpotID:         b.ParkingSpotID,
		ParkingSpotTitle:      spot.Title,
		OwnerUserID:           spot.OwnerID,
		DriverUserID:          b.UserID,
		Start:                 b.Start,
		End:                   b.End,
		Amount:                b.Amount,
		PaymentID:             b.PaymentID,
		OwnerResponseDeadline: b.OwnerResponseDeadline,
	}
}

// bookingMessageEvent is the wire shape of booking.message_sent -- deliberately separate
// from bookingEvent (no time_range/amount/payment_id concept for a chat message), but the
// same self-contained philosophy: it carries the recipient id and spot title directly, so
// notifications-service needs no follow-up lookup.
type bookingMessageEvent struct {
	EventID          string    `json:"event_id"`
	EventType        string    `json:"event_type"`
	OccurredAt       time.Time `json:"occurred_at"`
	BookingID        string    `json:"booking_id"`
	ParkingSpotID    string    `json:"parking_spot_id"`
	ParkingSpotTitle string    `json:"parking_spot_title"`
	SenderUserID     string    `json:"sender_user_id"`
	RecipientUserID  string    `json:"recipient_user_id"`
	Body             string    `json:"body"`
}

func newBookingMessageEvent(message entity.BookingMessage, spotTitle, spotID, recipientID string) bookingMessageEvent {
	return bookingMessageEvent{
		EventID:          uuid.NewString(),
		EventType:        RoutingKeyBookingMessageSent,
		OccurredAt:       time.Now().UTC(),
		BookingID:        message.BookingID,
		ParkingSpotID:    spotID,
		ParkingSpotTitle: spotTitle,
		SenderUserID:     message.SenderID,
		RecipientUserID:  recipientID,
		Body:             message.Body,
	}
}

// supportTicketRepliedEvent is the wire shape of support.ticket_replied -- fired only when
// ParkLink staff reply to a ticket (a user replying to their own ticket has no one to notify:
// staff work off admin-dashboard, not push notifications). Self-contained like the other
// booking.* events: notifications-service needs no follow-up lookup to build the push.
type supportTicketRepliedEvent struct {
	EventID         string    `json:"event_id"`
	EventType       string    `json:"event_type"`
	OccurredAt      time.Time `json:"occurred_at"`
	TicketID        string    `json:"ticket_id"`
	BookingID       string    `json:"booking_id"`
	TicketSubject   string    `json:"ticket_subject"`
	RecipientUserID string    `json:"recipient_user_id"`
	Body            string    `json:"body"`
}

func newSupportTicketRepliedEvent(message entity.SupportTicketMessage, ticket entity.SupportTicket) supportTicketRepliedEvent {
	return supportTicketRepliedEvent{
		EventID:         uuid.NewString(),
		EventType:       RoutingKeySupportTicketReplied,
		OccurredAt:      time.Now().UTC(),
		TicketID:        ticket.ID,
		BookingID:       ticket.BookingID,
		TicketSubject:   ticket.Subject,
		RecipientUserID: ticket.CreatedBy,
		Body:            message.Body,
	}
}

// payoutPaidEvent is the wire shape of payout.paid -- fired once ParkLink staff mark a
// payout as PAID (see MarkPayoutPaidUseCase). Self-contained like every other event in this
// file: notifications-service needs no follow-up lookup to tell the owner their transfer
// went out and how much it was for.
type payoutPaidEvent struct {
	EventID          string    `json:"event_id"`
	EventType        string    `json:"event_type"`
	OccurredAt       time.Time `json:"occurred_at"`
	PayoutID         string    `json:"payout_id"`
	BookingID        string    `json:"booking_id"`
	RecipientUserID  string    `json:"recipient_user_id"`
	NetAmount        int       `json:"net_amount"`
	ParkingSpotTitle string    `json:"parking_spot_title"`
}

func newPayoutPaidEvent(payout entity.Payout, spotTitle string) payoutPaidEvent {
	return payoutPaidEvent{
		EventID:          uuid.NewString(),
		EventType:        RoutingKeyPayoutPaid,
		OccurredAt:       time.Now().UTC(),
		PayoutID:         payout.ID,
		BookingID:        payout.BookingID,
		RecipientUserID:  payout.OwnerID,
		NetAmount:        payout.NetAmount,
		ParkingSpotTitle: spotTitle,
	}
}

// emitBookingEvent publishes e under routingKey when a publisher is configured. A publish
// failure is swallowed on purpose: the booking transition it announces has already been
// committed and is the source of truth -- a lost notification degrades the experience but
// must not fail the request that triggered it. The publisher logs the failure. e is any of
// this file's *Event payload structs (bookingEvent, bookingMessageEvent, ...).
func emitBookingEvent(ctx context.Context, publisher EventPublisher, routingKey string, e any) {
	if publisher == nil {
		return
	}
	_ = publisher.Publish(ctx, routingKey, e)
}
