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

func TestGetTicketUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getTicketUseCaseSuite))
}

type getTicketUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockTicketGetter
	useCase    *booking.GetTicketUseCase
	context    context.Context

	ticketID string
	creator  string
}

func (s *getTicketUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockTicketGetter(s.controller)
	s.useCase = booking.NewGetTicketUseCase(s.tickets)
	s.context = context.Background()

	s.ticketID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.creator = "22222222-2222-2222-2222-222222222222"
}

func (s *getTicketUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getTicketUseCaseSuite) TestExecute_Creator_ReturnsTicket() {
	want := entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator}
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.ticketID, s.creator)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *getTicketUseCaseSuite) TestExecute_NotCreator_Forbidden() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator}, nil)

	_, err := s.useCase.Execute(s.context, s.ticketID, "someone-else")

	s.True(errors.IsForbidden(err))
}

func (s *getTicketUseCaseSuite) TestExecute_TicketNotFound_ReturnsNotFoundError() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{}, errors.NewNotFound("support_ticket", s.ticketID))

	_, err := s.useCase.Execute(s.context, s.ticketID, s.creator)

	s.True(errors.IsNotFound(err))
}
