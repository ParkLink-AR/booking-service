#!/usr/bin/env bash
# setup.sh — download the two external packages that require internet access.
# Run this once before building:
#   bash scripts/setup.sh

set -e

echo "Downloading PostgreSQL driver..."
go get github.com/lib/pq@v1.10.9

echo "Downloading RabbitMQ client..."
go get github.com/rabbitmq/amqp091-go@v1.10.0

echo "Tidying module graph..."
go mod tidy

echo "Done! You can now run: go build ./..."
