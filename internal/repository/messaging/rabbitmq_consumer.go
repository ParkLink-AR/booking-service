package messaging

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const queueName = "events.notifications"

// Consumer listens for events on a RabbitMQ queue and logs them.
type Consumer struct {
	channel *amqp.Channel
	logger  *zap.Logger
}

func NewConsumer(channel *amqp.Channel, logger *zap.Logger) *Consumer {
	return &Consumer{channel: channel, logger: logger}
}

// Start begins consuming messages from the events.notifications queue.
// It blocks until ctx is cancelled. Each message is acknowledged and logged.
func (consumer *Consumer) Start(ctx context.Context) {
	messages, err := consumer.channel.Consume(
		queueName,
		"",    // consumer tag
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		consumer.logger.Error("failed to start consumer", zap.Error(err))
		return
	}

	consumer.logger.Info("event consumer started", zap.String("queue", queueName))

	for {
		select {
		case <-ctx.Done():
			consumer.logger.Info("event consumer stopped")
			return
		case message, ok := <-messages:
			if !ok {
				consumer.logger.Warn("consumer channel closed, stopping")
				return
			}
			consumer.logger.Info("event received",
				zap.String("routing_key", message.RoutingKey),
				zap.ByteString("body", message.Body),
			)
		}
	}
}
