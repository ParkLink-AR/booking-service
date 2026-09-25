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

func TestListOwnerBookingsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listOwnerBookingsUseCaseSuite))
}

type listOwnerBookingsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	spots      *mocks.MockOwnerSpotIDsLookup
	bookings   *mocks.MockOwnerSpotBookingLister
	useCase    *booking.ListOwnerBookingsUseCase
	context    context.Context
	ownerID    string
}

func (s *listOwnerBookingsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.spots = mocks.NewMockOwnerSpotIDsLookup(s.controller)
	s.bookings = mocks.NewMockOwnerSpotBookingLister(s.controller)
	s.useCase = booking.NewListOwnerBookingsUseCase(s.spots, s.bookings)
	s.context = context.Background()
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *listOwnerBookingsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listOwnerBookingsUseCaseSuite) TestExecute_OwnerHasSpots_ReturnsBookingsAcrossAll() {
	spotIDs := []string{"spot1", "spot2"}
	want := []entity.Booking{{ID: "b1"}, {ID: "b2"}}
	s.spots.EXPECT().GetSpotIDsForOwner(gomock.Any(), s.ownerID).Return(spotIDs, nil)
	s.bookings.EXPECT().ListBookingsByOwnerSpots(gomock.Any(), spotIDs).Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listOwnerBookingsUseCaseSuite) TestExecute_OwnerHasNoSpots_ReturnsEmptyWithoutQueryingBookings() {
	s.spots.EXPECT().GetSpotIDsForOwner(gomock.Any(), s.ownerID).Return([]string{}, nil)

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Require().NoError(err)
	s.Empty(got)
}

func (s *listOwnerBookingsUseCaseSuite) TestExecute_SpotLookupFails_Propagates() {
	s.spots.EXPECT().GetSpotIDsForOwner(gomock.Any(), s.ownerID).
		Return(nil, errors.NewUpstream("parkingclient.GetSpotIDsForOwner", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Nil(got)
	s.True(errors.IsUpstream(err))
}
