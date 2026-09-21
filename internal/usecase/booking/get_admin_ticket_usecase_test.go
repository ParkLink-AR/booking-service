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

func TestGetAdminTicketUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getAdminTicketUseCaseSuite))
}

type getAdminTicketUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockTicketGetter
	getter     *mocks.MockBookingGetter
	spots      *mocks.MockSpotDetailsLookup
	useCase    *booking.GetAdminTicketUseCase
	context    context.Context

	ticketID  string
	bookingID string
}

func (s *getAdminTicketUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockTicketGetter(s.controller)
	s.getter = mocks.NewMockBookingGetter(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.useCase = booking.NewGetAdminTicketUseCase(s.tickets, s.getter, s.spots)
	s.context = context.Background()

	s.ticketID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.bookingID = "booking-1"
}

func (s *getAdminTicketUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getAdminTicketUseCaseSuite) TestExecute_EnrichesWithBookingAndSpot() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, BookingID: s.bookingID}, nil)
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{ID: s.bookingID, ParkingSpotID: "spot-1"}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").
		Return(entity.SpotDetails{Title: "Cochera Palermo"}, nil)

	ticket, _, spot, err := s.useCase.Execute(s.context, s.ticketID)

	s.Require().NoError(err)
	s.Equal(s.ticketID, ticket.ID)
	s.Equal("Cochera Palermo", spot.Title)
}

func (s *getAdminTicketUseCaseSuite) TestExecute_TicketNotFound_ReturnsNotFoundError() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{}, errors.NewNotFound("support_ticket", s.ticketID))

	_, _, _, err := s.useCase.Execute(s.context, s.ticketID)

	s.True(errors.IsNotFound(err))
}
