package parkingservice_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/service/parkingservice"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(serviceSuite))
}

type serviceSuite struct {
	suite.Suite
	controller               *gomock.Controller
	parkingServiceRepository *mocks.MockParkingServiceRepository
	service                  *parkingservice.Service
	context                  context.Context
}

func (s *serviceSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.parkingServiceRepository = mocks.NewMockParkingServiceRepository(s.controller)
	s.service = parkingservice.NewService(s.parkingServiceRepository)
	s.context = context.Background()
}

func (s *serviceSuite) TearDownTest() {
	s.controller.Finish()
}

// The service is a pure pass-through: it must forward the call and return exactly what
// the repository returns, with no logic of its own.
func (s *serviceSuite) TestGetAvailabilityBase_PassesThroughToRepository() {
	parkingSpotID := "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	want := &entity.AvailabilityBase{Days: []entity.DayBase{{Date: from}}}

	s.parkingServiceRepository.EXPECT().
		GetAvailabilityBase(gomock.Any(), parkingSpotID, from, to).
		Return(want, nil)

	got, err := s.service.GetAvailabilityBase(s.context, parkingSpotID, from, to)

	s.Require().NoError(err)
	s.Same(want, got)
}

func (s *serviceSuite) TestGetAvailabilityBase_PropagatesRepositoryError() {
	parkingSpotID := "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)

	s.parkingServiceRepository.EXPECT().
		GetAvailabilityBase(gomock.Any(), parkingSpotID, from, to).
		Return(nil, repositoryFailure)

	got, err := s.service.GetAvailabilityBase(s.context, parkingSpotID, from, to)

	s.Nil(got)
	s.ErrorIs(err, repositoryFailure)
}

var repositoryFailure = repositoryFailureFixture{}

type repositoryFailureFixture struct{}

func (repositoryFailureFixture) Error() string { return "parking-service unreachable" }
