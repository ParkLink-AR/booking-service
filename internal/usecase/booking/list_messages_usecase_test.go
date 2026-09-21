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

func TestListMessagesUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listMessagesUseCaseSuite))
}

type listMessagesUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	getter     *mocks.MockBookingGetter
	owners     *mocks.MockSpotOwnerLookup
	lister     *mocks.MockMessageLister
	useCase    *booking.ListMessagesUseCase
	context    context.Context

	bookingID string
	driverID  string
	ownerID   string
}

func (s *listMessagesUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.getter = mocks.NewMockBookingGetter(s.controller)
	s.owners = mocks.NewMockSpotOwnerLookup(s.controller)
	s.lister = mocks.NewMockMessageLister(s.controller)
	s.useCase = booking.NewListMessagesUseCase(s.getter, s.owners, s.lister)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *listMessagesUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listMessagesUseCaseSuite) booking() entity.Booking {
	return entity.Booking{ID: s.bookingID, UserID: s.driverID, ParkingSpotID: "spot-1"}
}

func (s *listMessagesUseCaseSuite) TestExecute_Driver_DoesNotNeedOwnerLookup() {
	messages := []entity.BookingMessage{{ID: "msg-1", BookingID: s.bookingID}}
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.lister.EXPECT().ListMessagesByBooking(gomock.Any(), s.bookingID).Return(messages, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Require().NoError(err)
	s.Equal(messages, got)
}

func (s *listMessagesUseCaseSuite) TestExecute_Owner_ChecksSpotOwnership() {
	messages := []entity.BookingMessage{{ID: "msg-1", BookingID: s.bookingID}}
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return(s.ownerID, nil)
	s.lister.EXPECT().ListMessagesByBooking(gomock.Any(), s.bookingID).Return(messages, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID)

	s.Require().NoError(err)
	s.Equal(messages, got)
}

func (s *listMessagesUseCaseSuite) TestExecute_Stranger_Forbidden() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return(s.ownerID, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, "someone-else")

	s.True(errors.IsForbidden(err))
}

func (s *listMessagesUseCaseSuite) TestExecute_BookingNotFound_ReturnsNotFoundError() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.True(errors.IsNotFound(err))
}
