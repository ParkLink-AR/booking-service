package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestCompleteFinishedBookingsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(completeFinishedBookingsUseCaseSuite))
}

type completeFinishedBookingsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	bookings   *mocks.MockFinishedBookingCompleter
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.CompleteFinishedBookingsUseCase
	context    context.Context
}

func (s *completeFinishedBookingsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookings = mocks.NewMockFinishedBookingCompleter(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewCompleteFinishedBookingsUseCase(s.bookings, s.spots, s.events)
	s.context = context.Background()
}

func (s *completeFinishedBookingsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *completeFinishedBookingsUseCaseSuite) TestExecute_AnnouncesEachCompletedStay() {
	s.bookings.EXPECT().ExpireCompletedBookings(gomock.Any()).Return([]entity.Booking{
		{ID: "b1", ParkingSpotID: "spot-1", UserID: "driver-1"},
	}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: "owner-1", Title: "Cochera Palermo"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCompleted, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal("booking.completed", event["event_type"])
			s.Equal("b1", event["booking_id"])
			s.Equal("driver-1", event["driver_user_id"])
			s.Equal("owner-1", event["owner_user_id"])
			s.Equal("Cochera Palermo", event["parking_spot_title"])
			return nil
		},
	)

	n, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(1, n)
}

func (s *completeFinishedBookingsUseCaseSuite) TestExecute_SweepFails_ReturnsError() {
	s.bookings.EXPECT().ExpireCompletedBookings(gomock.Any()).Return(nil, errors.NewInternalError("test", context.DeadlineExceeded))

	_, err := s.useCase.Execute(s.context)

	s.Error(err)
}
