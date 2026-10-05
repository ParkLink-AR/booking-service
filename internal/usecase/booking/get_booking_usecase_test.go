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

func TestGetBookingUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getBookingUseCaseSuite))
}

type getBookingUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	getter     *mocks.MockBookingGetter
	owners     *mocks.MockSpotOwnerLookup
	useCase    *booking.GetBookingUseCase
	context    context.Context
	stored     entity.Booking
}

func (s *getBookingUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.getter = mocks.NewMockBookingGetter(s.controller)
	s.owners = mocks.NewMockSpotOwnerLookup(s.controller)
	s.useCase = booking.NewGetBookingUseCase(s.getter, s.owners)
	s.context = context.Background()
	s.stored = entity.Booking{ID: "b-1", ParkingSpotID: "spot-1", UserID: "driver-1"}
}

func (s *getBookingUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getBookingUseCaseSuite) TestExecute_DriverCanRead() {
	s.getter.EXPECT().GetByID(gomock.Any(), "b-1").Return(s.stored, nil)

	got, err := s.useCase.Execute(s.context, "b-1", entity.BookingViewer{UserID: "driver-1"})

	s.Require().NoError(err)
	s.Equal(s.stored, got)
}

func (s *getBookingUseCaseSuite) TestExecute_SpotOwnerCanRead() {
	s.getter.EXPECT().GetByID(gomock.Any(), "b-1").Return(s.stored, nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return("owner-1", nil)

	_, err := s.useCase.Execute(s.context, "b-1", entity.BookingViewer{UserID: "owner-1"})

	s.Require().NoError(err)
}

func (s *getBookingUseCaseSuite) TestExecute_StrangerIsForbidden() {
	s.getter.EXPECT().GetByID(gomock.Any(), "b-1").Return(s.stored, nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return("owner-1", nil)

	_, err := s.useCase.Execute(s.context, "b-1", entity.BookingViewer{UserID: "stranger"})

	s.True(errors.IsForbidden(err))
}

func (s *getBookingUseCaseSuite) TestExecute_AnonymousIsForbidden() {
	s.getter.EXPECT().GetByID(gomock.Any(), "b-1").Return(s.stored, nil)

	_, err := s.useCase.Execute(s.context, "b-1", entity.BookingViewer{})

	s.True(errors.IsForbidden(err))
}

func (s *getBookingUseCaseSuite) TestExecute_PrivilegedCanRead() {
	s.getter.EXPECT().GetByID(gomock.Any(), "b-1").Return(s.stored, nil)

	_, err := s.useCase.Execute(s.context, "b-1", entity.BookingViewer{Privileged: true})

	s.Require().NoError(err)
}
