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
	"github.com/example/bookings-service/internal/service/availability"
	"github.com/example/bookings-service/pkg/mocks"
)

const maxRangeDays = 14

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(serviceSuite))
}

type serviceSuite struct {
	suite.Suite
	controller               *gomock.Controller
	activeBookingsRepository *mocks.MockActiveBookingsRepository
	service                  *availability.Service
	context                  context.Context
	location                 *time.Location
	parkingSpotID            string
}

func (s *serviceSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.activeBookingsRepository = mocks.NewMockActiveBookingsRepository(s.controller)
	s.service = availability.NewService(s.activeBookingsRepository, maxRangeDays)
	s.context = context.Background()

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location
	s.parkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *serviceSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *serviceSuite) localDate(day int) time.Time {
	return time.Date(2026, 8, day, 0, 0, 0, 0, s.location)
}

func (s *serviceSuite) TestGetAvailability_PartialBookingLeavesTwoFreeWindows() {
	from := s.localDate(18)
	to := s.localDate(19)

	baseWindow := entity.TimeWindow{
		Start: s.localDate(18).Add(8 * time.Hour),
		End:   s.localDate(18).Add(20 * time.Hour),
	}
	availabilityBase := &entity.AvailabilityBase{Days: []entity.DayBase{
		{Date: s.localDate(18), Windows: []entity.TimeWindow{baseWindow}},
	}}

	booking := entity.TimeWindow{
		Start: s.localDate(18).Add(10 * time.Hour).UTC(),
		End:   s.localDate(18).Add(11 * time.Hour).UTC(),
	}
	s.activeBookingsRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, from.UTC(), to.UTC()).
		Return([]entity.TimeWindow{booking}, nil)

	got, err := s.service.GetAvailability(s.context, s.parkingSpotID, from, to, availabilityBase)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal(enum.AvailabilityStatusAvailable, got[0].Status)
	s.Require().Len(got[0].FreeWindows, 2)
	s.Equal(8, got[0].FreeWindows[0].Start.In(s.location).Hour())
	s.Equal(10, got[0].FreeWindows[0].End.In(s.location).Hour())
	s.Equal(11, got[0].FreeWindows[1].Start.In(s.location).Hour())
	s.Equal(20, got[0].FreeWindows[1].End.In(s.location).Hour())
}

func (s *serviceSuite) TestGetAvailability_ClosedDaySkipsBookingSubtraction() {
	from := s.localDate(22) // Saturday
	to := s.localDate(23)

	availabilityBase := &entity.AvailabilityBase{Days: []entity.DayBase{
		{Date: s.localDate(22), Windows: []entity.TimeWindow{}},
	}}

	s.activeBookingsRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, from.UTC(), to.UTC()).
		Return(nil, nil)

	got, err := s.service.GetAvailability(s.context, s.parkingSpotID, from, to, availabilityBase)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal(enum.AvailabilityStatusClosed, got[0].Status)
	s.Empty(got[0].FreeWindows)
}

func (s *serviceSuite) TestGetAvailability_FullyBookedDayIsFull() {
	from := s.localDate(19)
	to := s.localDate(20)

	baseWindow := entity.TimeWindow{
		Start: s.localDate(19).Add(8 * time.Hour),
		End:   s.localDate(19).Add(20 * time.Hour),
	}
	availabilityBase := &entity.AvailabilityBase{Days: []entity.DayBase{
		{Date: s.localDate(19), Windows: []entity.TimeWindow{baseWindow}},
	}}

	booking := entity.TimeWindow{Start: baseWindow.Start.UTC(), End: baseWindow.End.UTC()}
	s.activeBookingsRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, from.UTC(), to.UTC()).
		Return([]entity.TimeWindow{booking}, nil)

	got, err := s.service.GetAvailability(s.context, s.parkingSpotID, from, to, availabilityBase)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal(enum.AvailabilityStatusFull, got[0].Status)
	s.Empty(got[0].FreeWindows)
}

func (s *serviceSuite) TestGetAvailability_InvalidDateRange() {
	from := s.localDate(19)
	to := s.localDate(18)

	got, err := s.service.GetAvailability(s.context, s.parkingSpotID, from, to, &entity.AvailabilityBase{})

	s.Nil(got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("INVALID_DATE_RANGE", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestGetAvailability_RangeTooLarge() {
	from := s.localDate(1)
	to := from.AddDate(0, 0, 15) // 15 days > max of 14

	got, err := s.service.GetAvailability(s.context, s.parkingSpotID, from, to, &entity.AvailabilityBase{})

	s.Nil(got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("RANGE_TOO_LARGE", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestGetAvailability_RepositoryErrorPropagates() {
	from := s.localDate(18)
	to := s.localDate(19)

	s.activeBookingsRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, from.UTC(), to.UTC()).
		Return(nil, repositoryFailure)

	got, err := s.service.GetAvailability(s.context, s.parkingSpotID, from, to, &entity.AvailabilityBase{})

	s.Nil(got)
	s.ErrorIs(err, repositoryFailure)
}

var repositoryFailure = repositoryFailureFixture{}

type repositoryFailureFixture struct{}

func (repositoryFailureFixture) Error() string { return "connection refused" }
