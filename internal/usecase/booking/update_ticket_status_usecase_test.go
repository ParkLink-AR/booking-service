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

func TestUpdateTicketStatusUseCaseSuite(t *testing.T) {
	suite.Run(t, new(updateTicketStatusUseCaseSuite))
}

type updateTicketStatusUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	status     *mocks.MockTicketStatusUpdater
	useCase    *booking.UpdateTicketStatusUseCase
	context    context.Context

	ticketID string
}

func (s *updateTicketStatusUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.status = mocks.NewMockTicketStatusUpdater(s.controller)
	s.useCase = booking.NewUpdateTicketStatusUseCase(s.status)
	s.context = context.Background()

	s.ticketID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *updateTicketStatusUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *updateTicketStatusUseCaseSuite) TestExecute_ValidStatus_Updates() {
	s.status.EXPECT().UpdateTicketStatus(gomock.Any(), s.ticketID, enum.TicketStatusResolved).
		Return(entity.SupportTicket{ID: s.ticketID, Status: enum.TicketStatusResolved}, nil)

	got, err := s.useCase.Execute(s.context, s.ticketID, "RESOLVED")

	s.Require().NoError(err)
	s.Equal(enum.TicketStatusResolved, got.Status)
}

func (s *updateTicketStatusUseCaseSuite) TestExecute_InvalidStatus_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.ticketID, "NOT_A_STATUS")

	s.True(errors.IsInvalidInput(err))
	s.Equal("INVALID_STATUS", errors.InvalidInputCode(err))
}

func (s *updateTicketStatusUseCaseSuite) TestExecute_TicketNotFound_ReturnsNotFoundError() {
	s.status.EXPECT().UpdateTicketStatus(gomock.Any(), s.ticketID, enum.TicketStatusClosed).
		Return(entity.SupportTicket{}, errors.NewNotFound("support_ticket", s.ticketID))

	_, err := s.useCase.Execute(s.context, s.ticketID, "CLOSED")

	s.True(errors.IsNotFound(err))
}
