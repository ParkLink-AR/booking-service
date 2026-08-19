package availability_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/availability"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestGetAvailabilityUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getAvailabilityUseCaseSuite))
}

type getAvailabilityUseCaseSuite struct {
	suite.Suite
	controller          *gomock.Controller
	parkingService      *mocks.MockParkingService
	availabilityService *mocks.MockAvailabilityService
	useCase             *availability.GetAvailabilityUseCase
	context             context.Context
	location            *time.Location
	parkingSpotID       string
}

func (s *getAvailabilityUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.parkingService = mocks.NewMockParkingService(s.controller)
	s.availabilityService = mocks.NewMockAvailabilityService(s.controller)
	s.useCase = availability.NewGetAvailabilityUseCase(s.parkingService, s.availabilityService)
	s.context = context.Background()

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location
	s.parkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *getAvailabilityUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getAvailabilityUseCaseSuite) localDate(day int) time.Time {
	return time.Date(2026, 8, day, 0, 0, 0, 0, s.location)
}

func (s *getAvailabilityUseCaseSuite) TestExecute_DelegatesToParkingServiceThenAvailabilityService() {
	from := s.localDate(18)
	to := s.localDate(19)

	availabilityBase := &entity.AvailabilityBase{Days: []entity.DayBase{
		{Date: s.localDate(18), Windows: []entity.TimeWindow{{Start: from, End: to}}},
	}}
	want := []entity.DayAvailability{
		{Date: s.localDate(18), Status: enum.AvailabilityStatusAvailable},
	}

	s.parkingService.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, from, to).
		Return(availabilityBase, nil)
	s.availabilityService.EXPECT().
		GetAvailability(gomock.Any(), s.parkingSpotID, from, to, availabilityBase).
		Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.parkingSpotID, from, to)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *getAvailabilityUseCaseSuite) TestExecute_ParkingServiceErrorWrapsAsUpstream() {
	from := s.localDate(18)
	to := s.localDate(19)

	s.parkingService.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, from, to).
		Return(nil, upstreamFailure)

	got, err := s.useCase.Execute(s.context, s.parkingSpotID, from, to)

	s.Nil(got)
	s.True(errors.IsUpstream(err))
}

func (s *getAvailabilityUseCaseSuite) TestExecute_AvailabilityServiceErrorPropagates() {
	from := s.localDate(18)
	to := s.localDate(19)

	availabilityBase := &entity.AvailabilityBase{}
	s.parkingService.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, from, to).
		Return(availabilityBase, nil)
	s.availabilityService.EXPECT().
		GetAvailability(gomock.Any(), s.parkingSpotID, from, to, availabilityBase).
		Return(nil, errors.NewInvalidInput("RANGE_TOO_LARGE", "date range exceeds maximum of 14 days"))

	got, err := s.useCase.Execute(s.context, s.parkingSpotID, from, to)

	s.Nil(got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("RANGE_TOO_LARGE", errors.InvalidInputCode(err))
}

var upstreamFailure = upstreamFailureFixture{}

type upstreamFailureFixture struct{}

func (upstreamFailureFixture) Error() string { return "parking-service unreachable" }
