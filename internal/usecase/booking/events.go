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
	// AutoConfirmed marks a booking.confirmed that skipped owner approval because the
	// spot is instant-booking -- the owner never acted, so consumers may still want to
	// notify them.
	AutoConfirmed bool `json:"auto_confirmed,omitempty"`
}

func newBookingEvent(eventType string, b entity.Booking, spot entity.SpotDetails, autoConfirmed bool) bookingEvent {
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
		AutoConfirmed:         autoConfirmed,
	}
}

// emitBookingEvent publishes e under routingKey when a publisher is configured. A publish
// failure is swallowed on purpose: the booking transition it announces has already been
// committed and is the source of truth -- a lost notification degrades the experience but
// must not fail the request that triggered it. The publisher logs the failure.
func emitBookingEvent(ctx context.Context, publisher EventPublisher, routingKey string, e bookingEvent) {
	if publisher == nil {
		return
	}
	_ = publisher.Publish(ctx, routingKey, e)
}
