package messaging

import (
	"context"

	"github.com/goccy/go-json"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	exchangeName = "events"
	routingKey   = "event.created"
)

// Publisher publishes domain events to RabbitMQ.
type Publisher struct {
	channel *amqp.Channel
	logger  *zap.Logger
}

func NewPublisher(channel *amqp.Channel, logger *zap.Logger) *Publisher {
	return &Publisher{channel: channel, logger: logger}
}

// Publish serialises payload as JSON and sends it to the events exchange.
func (publisher *Publisher) Publish(_ context.Context, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	err = publisher.channel.Publish(
		exchangeName,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		publisher.logger.Error("failed to publish event", zap.Error(err))
		return err
	}

	publisher.logger.Info("event published", zap.String("routing_key", routingKey))
	return nil
}
