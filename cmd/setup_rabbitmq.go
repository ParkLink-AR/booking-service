package main

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitExchangeName = "parklink.events"
	rabbitQueueName    = "events.notifications"
	rabbitBindingKey   = "#"
)

// setupRabbitMQ establishes a connection, declares the exchange/queue, and
// returns the connection and a channel ready for publishing and consuming.
func setupRabbitMQ() (*amqp.Connection, *amqp.Channel) {
	url := envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	connection, err := amqp.Dial(url)
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
