package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestListBookingsDetailedUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listBookingsDetailedUseCaseSuite))
}

type listBookingsDetailedUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	lister     *mocks.MockBookingListerBySpot
	owners     *mocks.MockSpotOwnerLookup
	useCase    *booking.ListBookingsDetailedUseCase
	context    context.Context

	spotID  string
	ownerID string
}

func (s *listBookingsDetailedUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.lister = mocks.NewMockBookingListerBySpot(s.controller)
	s.owners = mocks.NewMockSpotOwnerLookup(s.controller)
	s.useCase = booking.NewListBookingsDetailedUseCase(s.lister, s.owners)
	s.context = context.Background()

	s.spotID = "11111111-1111-1111-1111-111111111111"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *listBookingsDetailedUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listBookingsDetailedUseCaseSuite) TestHidesBookingsThatNeverWentThrough() {
	s.owners.EXPECT().GetSpotOwnerID(s.context, s.spotID).Return(s.ownerID, nil)
	s.lister.EXPECT().ListBookingsBySpot(s.context, s.spotID).Return([]entity.Booking{
		{ID: "pending-payment", Status: enum.BookingStatusPendingPayment},
		{ID: "pending-approval", Status: enum.BookingStatusPendingOwnerApproval},
		{ID: "confirmed", Status: enum.BookingStatusConfirmed},
		{ID: "in-progress", Status: enum.BookingStatusInProgress},
		{ID: "expired", Status: enum.BookingStatusExpired},
		{ID: "completed", Status: enum.BookingStatusCompleted},
		{ID: "cancelled", Status: enum.BookingStatusCancelled},
	}, nil)

	bookings, err := s.useCase.Execute(s.context, s.spotID, s.ownerID)

	s.Require().NoError(err)
	ids := make([]string, len(bookings))
	for i, b := range bookings {
		ids[i] = b.ID
	}
	s.Equal([]string{"pending-approval", "confirmed", "in-progress", "completed", "cancelled"}, ids)
}

func (s *listBookingsDetailedUseCaseSuite) TestRejectsCallerWhoIsNotTheOwner() {
	s.owners.EXPECT().GetSpotOwnerID(s.context, s.spotID).Return(s.ownerID, nil)

	bookings, err := s.useCase.Execute(s.context, s.spotID, "someone-else")

	s.Nil(bookings)
	var forbidden *errors.ForbiddenError
	s.Require().ErrorAs(err, &forbidden)
}
