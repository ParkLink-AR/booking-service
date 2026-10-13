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

func TestAddTicketMessageUseCaseSuite(t *testing.T) {
	suite.Run(t, new(addTicketMessageUseCaseSuite))
}

type addTicketMessageUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockTicketGetter
	messages   *mocks.MockTicketMessageCreator
	status     *mocks.MockTicketStatusUpdater
	bookings   *mocks.MockTicketBookingGetter
	events     *mocks.MockEventPublisher
	useCase    *booking.AddTicketMessageUseCase
	context    context.Context

	ticketID string
	creator  string
}

func (s *addTicketMessageUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockTicketGetter(s.controller)
	s.messages = mocks.NewMockTicketMessageCreator(s.controller)
	s.status = mocks.NewMockTicketStatusUpdater(s.controller)
	s.bookings = mocks.NewMockTicketBookingGetter(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewAddTicketMessageUseCase(s.tickets, s.messages, s.status, s.bookings, s.events)
	s.context = context.Background()

	s.ticketID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.creator = "22222222-2222-2222-2222-222222222222"
}

func (s *addTicketMessageUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *addTicketMessageUseCaseSuite) TestExecute_Creator_AddsMessage() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator}, nil)
	s.messages.EXPECT().CreateTicketMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, m entity.SupportTicketMessage) (entity.SupportTicketMessage, error) {
			s.Equal(s.creator, m.SenderID)
			s.False(m.IsAdmin)
			m.ID = "msg-1"
			return m, nil
		},
	)

	s.bookings.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(entity.Booking{UserID: s.creator}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeySupportTicketUserMessage, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal("¿Alguna novedad?", event["body"])
			s.Equal(booking.RecipientRoleDriver, event["author_role"])
			return nil
		},
	)

	got, err := s.useCase.Execute(s.context, s.ticketID, s.creator, "¿Alguna novedad?")

	s.Require().NoError(err)
	s.Equal("msg-1", got.ID)
}

// The problem wasn't solved after all: writing on a resolved ticket reopens it.
func (s *addTicketMessageUseCaseSuite) TestExecute_ResolvedTicket_Reopens() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator, Status: enum.TicketStatusResolved}, nil)
	s.messages.EXPECT().CreateTicketMessage(gomock.Any(), gomock.Any()).Return(entity.SupportTicketMessage{ID: "msg-1"}, nil)
	s.status.EXPECT().UpdateTicketStatus(gomock.Any(), s.ticketID, enum.TicketStatusOpen).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator, Status: enum.TicketStatusOpen}, nil)
	s.bookings.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(entity.Booking{}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeySupportTicketUserMessage, gomock.Any()).Return(nil)

	_, err := s.useCase.Execute(s.context, s.ticketID, s.creator, "Sigue sin andar")

	s.Require().NoError(err)
}

// A closed ticket is final: no more messages (the app points to opening a new one).
func (s *addTicketMessageUseCaseSuite) TestExecute_ClosedTicket_Refused() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator, Status: enum.TicketStatusClosed}, nil)

	_, err := s.useCase.Execute(s.context, s.ticketID, s.creator, "hola")

	s.Equal("TICKET_CLOSED", errors.InvalidInputCode(err))
}

func (s *addTicketMessageUseCaseSuite) TestExecute_NotCreator_Forbidden() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{ID: s.ticketID, CreatedBy: s.creator}, nil)

	_, err := s.useCase.Execute(s.context, s.ticketID, "someone-else", "hola")

	s.True(errors.IsForbidden(err))
}

func (s *addTicketMessageUseCaseSuite) TestExecute_EmptyBody_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.ticketID, s.creator, "   ")

	s.True(errors.IsInvalidInput(err))
	s.Equal("EMPTY_MESSAGE", errors.InvalidInputCode(err))
}

func (s *addTicketMessageUseCaseSuite) TestExecute_TooLong_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.ticketID, s.creator, strings.Repeat("a", 2001))

	s.True(errors.IsInvalidInput(err))
	s.Equal("MESSAGE_TOO_LONG", errors.InvalidInputCode(err))
}

func (s *addTicketMessageUseCaseSuite) TestExecute_TicketNotFound_ReturnsNotFoundError() {
	s.tickets.EXPECT().GetTicketByID(gomock.Any(), s.ticketID).
		Return(entity.SupportTicket{}, errors.NewNotFound("support_ticket", s.ticketID))

	_, err := s.useCase.Execute(s.context, s.ticketID, s.creator, "hola")

	s.True(errors.IsNotFound(err))
}
