// Package monitoring reports errors to Sentry. Without SENTRY_DSN it stays off: every call
// here is then a no-op, so development and tests send nothing.
package monitoring

import (
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

const serviceName = "booking-service"

// Init starts the Sentry client from SENTRY_DSN (and SENTRY_ENVIRONMENT, default
// "production"). Returns a flush to call on shutdown, so queued events still go out.
func Init() (func(), error) {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return func() {}, nil
	}
	environment := os.Getenv("SENTRY_ENVIRONMENT")
	if environment == "" {
		environment = "production"
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: environment,
		ServerName:  serviceName,
	}); err != nil {
		return func() {}, err
	}
	sentry.ConfigureScope(func(scope *sentry.Scope) { scope.SetTag("service", serviceName) })
	return func() { sentry.Flush(2 * time.Second) }, nil
}

// Middleware reports a panic in a handler (with the request) and lets it carry on to the
// router's own recoverer, which answers 500.
func Middleware(next http.Handler) http.Handler {
	return sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(next)
}

// CaptureError reports an unexpected error -- one that ended a request in a 5xx.
func CaptureError(err error) {
	if err != nil {
		sentry.CaptureException(err)
	}
}
