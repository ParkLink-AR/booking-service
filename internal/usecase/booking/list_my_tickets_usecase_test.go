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

func TestListMyTicketsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listMyTicketsUseCaseSuite))
}

type listMyTicketsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	getter     *mocks.MockBookingGetter
	owners     *mocks.MockSpotOwnerLookup
	tickets    *mocks.MockTicketListerByBookingAndCreator
	useCase    *booking.ListMyTicketsUseCase
	context    context.Context

	bookingID string
	driverID  string
	ownerID   string
}

func (s *listMyTicketsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.getter = mocks.NewMockBookingGetter(s.controller)
	s.owners = mocks.NewMockSpotOwnerLookup(s.controller)
	s.tickets = mocks.NewMockTicketListerByBookingAndCreator(s.controller)
	s.useCase = booking.NewListMyTicketsUseCase(s.getter, s.owners, s.tickets)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *listMyTicketsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listMyTicketsUseCaseSuite) booking() entity.Booking {
	return entity.Booking{ID: s.bookingID, UserID: s.driverID, ParkingSpotID: "spot-1"}
}

func (s *listMyTicketsUseCaseSuite) TestExecute_Driver_ReturnsOwnTickets() {
	want := []entity.SupportTicket{{ID: "ticket-1", Status: enum.TicketStatusOpen}}
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.tickets.EXPECT().ListTicketsByBookingAndCreator(gomock.Any(), s.bookingID, s.driverID).Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listMyTicketsUseCaseSuite) TestExecute_Owner_ReturnsOwnTickets() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return(s.ownerID, nil)
	s.tickets.EXPECT().ListTicketsByBookingAndCreator(gomock.Any(), s.bookingID, s.ownerID).Return(nil, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID)

	s.Require().NoError(err)
}

func (s *listMyTicketsUseCaseSuite) TestExecute_Stranger_Forbidden() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return(s.ownerID, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, "someone-else")

	s.True(errors.IsForbidden(err))
}

func (s *listMyTicketsUseCaseSuite) TestExecute_BookingNotFound_ReturnsNotFoundError() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.True(errors.IsNotFound(err))
}
