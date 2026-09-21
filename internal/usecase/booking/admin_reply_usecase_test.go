package booking_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestAdminReplyUseCaseSuite(t *testing.T) {
	suite.Run(t, new(adminReplyUseCaseSuite))
}

type adminReplyUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockTicketGetter
	messages   *mocks.MockTicketMessageCreator
	status     *mocks.MockTicketStatusUpdater
	events     *mocks.MockEventPublisher
	useCase    *booking.AdminReplyUseCase
	context    context.Context

	ticketID string
	adminID  string
}

func (s *adminReplyUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockTicketGetter(s.controller)
	s.messages = mocks.NewMockTicketMessageCreator(s.controller)
	s.status = mocks.NewMockTicketStatusUpdater(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewAdminReplyUseCase(s.tickets, s.messages, s.status, s.events)
	s.context = context.Background()

	s.ticketID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.adminID = "admin-1"
}

func (s *adminReplyUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *adminReplyUseCaseSuite) TestExecute_OpenTicket_RepliesAndBumpsToInProgress() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, Status: enum.TicketStatusOpen}, nil)
	s.messages.EXPECT().CreateTicketMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, m entity.SupportTicketMessage) (entity.SupportTicketMessage, error) {
			s.Equal(s.adminID, m.SenderID)
			s.True(m.IsAdmin)
			m.ID = "msg-1"
			return m, nil
		},
	)
	s.status.EXPECT().UpdateTicketStatus(gomock.Any(), s.ticketID, enum.TicketStatusInProgress).
		Return(entity.SupportTicket{ID: s.ticketID, Status: enum.TicketStatusInProgress}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeySupportTicketReplied, gomock.Any()).Return(nil)

	got, err := s.useCase.Execute(s.context, s.ticketID, s.adminID, "Ya te reembolsamos el pago")

	s.Require().NoError(err)
	s.Equal("msg-1", got.ID)
}

func (s *adminReplyUseCaseSuite) TestExecute_ResolvedTicket_RepliesWithoutChangingStatus() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, Status: enum.TicketStatusResolved}, nil)
	s.messages.EXPECT().CreateTicketMessage(gomock.Any(), gomock.Any()).Return(entity.SupportTicketMessage{ID: "msg-1"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeySupportTicketReplied, gomock.Any()).Return(nil)

	_, err := s.useCase.Execute(s.context, s.ticketID, s.adminID, "Una aclaración más")

	s.Require().NoError(err)
}

func (s *adminReplyUseCaseSuite) TestExecute_EmptyBody_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.ticketID, s.adminID, "   ")

	s.True(errors.IsInvalidInput(err))
	s.Equal("EMPTY_MESSAGE", errors.InvalidInputCode(err))
}

func (s *adminReplyUseCaseSuite) TestExecute_TooLong_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.ticketID, s.adminID, strings.Repeat("a", 2001))

	s.True(errors.IsInvalidInput(err))
	s.Equal("MESSAGE_TOO_LONG", errors.InvalidInputCode(err))
}

func (s *adminReplyUseCaseSuite) TestExecute_TicketNotFound_ReturnsNotFoundError() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{}, errors.NewNotFound("support_ticket", s.ticketID))

	_, err := s.useCase.Execute(s.context, s.ticketID, s.adminID, "hola")

	s.True(errors.IsNotFound(err))
}

// A publish failure must not fail the request: the reply is already saved.
func (s *adminReplyUseCaseSuite) TestExecute_PublishFailureIsSwallowed() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, Status: enum.TicketStatusInProgress}, nil)
	s.messages.EXPECT().CreateTicketMessage(gomock.Any(), gomock.Any()).Return(entity.SupportTicketMessage{ID: "msg-1"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeySupportTicketReplied, gomock.Any()).
		Return(errors.NewInternalError("test", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.ticketID, s.adminID, "hola")

	s.Require().NoError(err)
	s.Equal("msg-1", got.ID)
}
