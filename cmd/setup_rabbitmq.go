package main

import (
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitExchangeName = "parklink.events"
	rabbitQueueName    = "events.notifications"
	rabbitBindingKey   = "#"

	rabbitDialAttempts = 10
	rabbitDialDelay    = 2 * time.Second
)

// setupRabbitMQ establishes a connection, declares the exchange/queue, and
// returns the connection and a channel ready for publishing and consuming.
func setupRabbitMQ() (*amqp.Connection, *amqp.Channel) {
	url := envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	connection, err := dialRabbitMQWithRetry(url, rabbitDialAttempts, rabbitDialDelay)
	if err != nil {
		log.Fatalf("rabbitmq dial: %v", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		log.Fatalf("rabbitmq channel: %v", err)
	}

	// Declare a durable topic exchange.
	if err := channel.ExchangeDeclare(rabbitExchangeName, "topic", true, false, false, false, nil); err != nil {
		log.Fatalf("exchange declare: %v", err)
	}

	// Declare a durable queue for notifications.
	if _, err := channel.QueueDeclare(rabbitQueueName, true, false, false, false, nil); err != nil {
		log.Fatalf("queue declare: %v", err)
	}

	// Bind the queue to the exchange.
	if err := channel.QueueBind(rabbitQueueName, rabbitBindingKey, rabbitExchangeName, false, nil); err != nil {
		log.Fatalf("queue bind: %v", err)
	}

	log.Printf("rabbitmq ready — exchange: %s, queue: %s", rabbitExchangeName, rabbitQueueName)
	return connection, channel
}

// dialRabbitMQWithRetry retries the initial dial for up to attempts*delay.
// docker compose's `condition: service_healthy` only waits for RabbitMQ's
// own healthcheck, which can report healthy a moment before the AMQP
// listener is actually accepting connections -- the very first dial right
// after startup can still lose that race and get connection refused.
func dialRabbitMQWithRetry(url string, attempts int, delay time.Duration) (*amqp.Connection, error) {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		connection, err := amqp.Dial(url)
		if err == nil {
			return connection, nil
		}
		lastErr = err
		log.Printf("rabbitmq dial attempt %d/%d failed: %v", attempt, attempts, err)
		if attempt < attempts {
			time.Sleep(delay)
		}
	}
	return nil, fmt.Errorf("unreachable after %d attempts: %w", attempts, lastErr)
}
