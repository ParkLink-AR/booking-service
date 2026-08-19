module github.com/example/bookings-service

go 1.25.0

require (
	github.com/go-chi/chi/v5 v5.0.11
	github.com/goccy/go-json v0.10.2
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
	github.com/rabbitmq/amqp091-go v1.10.0
	github.com/stretchr/testify v1.9.0
	go.uber.org/mock v0.4.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.11.0 // indirect
	golang.org/x/sys v0.1.0 // indirect
	golang.org/x/tools v0.2.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Run `bash scripts/setup.sh` to add the infrastructure packages:
//   github.com/lib/pq v1.10.9         (PostgreSQL driver)
//   github.com/rabbitmq/amqp091-go v1.10.0 (RabbitMQ client)

tool go.uber.org/mock/mockgen
