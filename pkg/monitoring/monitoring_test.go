package monitoring_test

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/pkg/monitoring"
)

func TestMonitoringSuite(t *testing.T) {
	suite.Run(t, new(monitoringSuite))
}

type monitoringSuite struct {
	suite.Suite
}

func (s *monitoringSuite) TestInit_WithoutDSN_StaysOff() {
	s.T().Setenv("SENTRY_DSN", "")

	flush, err := monitoring.Init()

	s.Require().NoError(err)
	flush()
}

func (s *monitoringSuite) TestInit_WithDSN_StartsTheClient() {
	s.T().Setenv("SENTRY_DSN", "https://abc123@o0.ingest.sentry.io/0")

	flush, err := monitoring.Init()

	s.Require().NoError(err)
	s.NotNil(sentry.CurrentHub().Client())
	flush()
}
