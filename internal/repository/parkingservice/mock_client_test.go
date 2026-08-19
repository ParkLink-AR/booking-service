package parkingservice_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/repository/parkingservice"
)

func TestMockClientSuite(t *testing.T) {
	suite.Run(t, new(mockClientSuite))
}

type mockClientSuite struct {
	suite.Suite
	location *time.Location
	client   *parkingservice.MockClient
	ctx      context.Context
}

func (s *mockClientSuite) SetupTest() {
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location
	s.client = parkingservice.NewMockClient(location)
	s.ctx = context.Background()
}

// 2026-08-18 is a Monday; the range below spans Mon-Thu (2026-08-21 excluded, "to" is exclusive).
func (s *mockClientSuite) TestGetAvailabilityBase_WeekdaysReturnFixedWindow() {
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, s.location)
	to := time.Date(2026, 8, 21, 0, 0, 0, 0, s.location)

	base, err := s.client.GetAvailabilityBase(s.ctx, "any-spot-id", from, to)

	s.Require().NoError(err)
	s.Require().Len(base.Days, 3)
	for i, day := range base.Days {
		s.Require().Len(day.Windows, 1, "day %d should have exactly one window", i)
		s.Equal(8, day.Windows[0].Start.Hour())
		s.Equal(20, day.Windows[0].End.Hour())
	}
}

// 2026-08-22 is a Saturday, 2026-08-23 is a Sunday.
func (s *mockClientSuite) TestGetAvailabilityBase_WeekendReturnsEmptyWindow() {
	from := time.Date(2026, 8, 22, 0, 0, 0, 0, s.location)
	to := time.Date(2026, 8, 24, 0, 0, 0, 0, s.location)

	base, err := s.client.GetAvailabilityBase(s.ctx, "any-spot-id", from, to)

	s.Require().NoError(err)
	s.Require().Len(base.Days, 2)
	for i, day := range base.Days {
		s.Empty(day.Windows, "day %d should have no windows", i)
	}
}

func (s *mockClientSuite) TestGetAvailabilityBase_ToDateIsExclusive() {
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, s.location)
	to := time.Date(2026, 8, 19, 0, 0, 0, 0, s.location)

	base, err := s.client.GetAvailabilityBase(s.ctx, "any-spot-id", from, to)

	s.Require().NoError(err)
	s.Require().Len(base.Days, 1)
	s.Equal(18, base.Days[0].Date.Day())
}
