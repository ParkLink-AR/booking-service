package messaging

import (
	"context"
	"time"

	"github.com/goccy/go-json"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// defaultEventsExchange is the shared topic exchange every ParkLink service publishes
// domain events to. Kept in sync with cmd/setup_rabbitmq.go's rabbitExchangeName and
// notifications-service's EVENTS_EXCHANGE.
const defaultEventsExchange = "parklink.events"

// Publisher publishes domain events to the shared `parklink.events` topic exchange.
type Publisher struct {
	channel  *amqp.Channel
	exchange string
	logger   *zap.Logger
}

func NewPublisher(channel *amqp.Channel, exchange string, logger *zap.Logger) *Publisher {
	if exchange == "" {
		exchange = defaultEventsExchange
	}
	return &Publisher{channel: channel, exchange: exchange, logger: logger}
}

// Publish serialises payload as JSON and sends it to the events exchange under
// routingKey (e.g. "booking.confirmed"). Messages are marked persistent so they survive
// a broker restart until a consumer acks them.
func (publisher *Publisher) Publish(_ context.Context, routingKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		publisher.logger.Error("failed to marshal event", zap.String("routing_key", routingKey), zap.Error(err))
		return err
	}

	err = publisher.channel.Publish(
		publisher.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		publisher.logger.Error("failed to publish event", zap.String("routing_key", routingKey), zap.Error(err))
		return err
	}

	publisher.logger.Info("event published", zap.String("routing_key", routingKey))
	return nil
}
