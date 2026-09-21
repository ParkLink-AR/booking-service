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

func TestListAdminTicketMessagesUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listAdminTicketMessagesUseCaseSuite))
}

type listAdminTicketMessagesUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockTicketGetter
	messages   *mocks.MockTicketMessageLister
	useCase    *booking.ListAdminTicketMessagesUseCase
	context    context.Context

	ticketID string
}

func (s *listAdminTicketMessagesUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockTicketGetter(s.controller)
	s.messages = mocks.NewMockTicketMessageLister(s.controller)
	s.useCase = booking.NewListAdminTicketMessagesUseCase(s.tickets, s.messages)
	s.context = context.Background()

	s.ticketID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *listAdminTicketMessagesUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listAdminTicketMessagesUseCaseSuite) TestExecute_ReturnsThreadRegardlessOfCreator() {
	want := []entity.SupportTicketMessage{{ID: "msg-1"}}
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: "not-the-caller"}, nil)
	s.messages.EXPECT().ListMessagesByTicket(gomock.Any(), s.ticketID).Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.ticketID)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listAdminTicketMessagesUseCaseSuite) TestExecute_TicketNotFound_ReturnsNotFoundError() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{}, errors.NewNotFound("support_ticket", s.ticketID))

	_, err := s.useCase.Execute(s.context, s.ticketID)

	s.True(errors.IsNotFound(err))
}
