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

func TestListTicketMessagesUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listTicketMessagesUseCaseSuite))
}

type listTicketMessagesUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockTicketGetter
	messages   *mocks.MockTicketMessageLister
	useCase    *booking.ListTicketMessagesUseCase
	context    context.Context

	ticketID string
	creator  string
}

func (s *listTicketMessagesUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockTicketGetter(s.controller)
	s.messages = mocks.NewMockTicketMessageLister(s.controller)
	s.useCase = booking.NewListTicketMessagesUseCase(s.tickets, s.messages)
	s.context = context.Background()

	s.ticketID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.creator = "22222222-2222-2222-2222-222222222222"
}

func (s *listTicketMessagesUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listTicketMessagesUseCaseSuite) TestExecute_Creator_ReturnsMessages() {
	want := []entity.SupportTicketMessage{{ID: "msg-1", TicketID: s.ticketID}}
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator}, nil)
	s.messages.EXPECT().ListMessagesByTicket(gomock.Any(), s.ticketID).Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.ticketID, s.creator)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listTicketMessagesUseCaseSuite) TestExecute_NotCreator_Forbidden() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator}, nil)

	_, err := s.useCase.Execute(s.context, s.ticketID, "someone-else")

	s.True(errors.IsForbidden(err))
}

func (s *listTicketMessagesUseCaseSuite) TestExecute_TicketNotFound_ReturnsNotFoundError() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{}, errors.NewNotFound("support_ticket", s.ticketID))

	_, err := s.useCase.Execute(s.context, s.ticketID, s.creator)

	s.True(errors.IsNotFound(err))
}
