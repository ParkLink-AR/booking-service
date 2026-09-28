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

func TestListTicketsByUserUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listTicketsByUserUseCaseSuite))
}

type listTicketsByUserUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockTicketListerByUser
	useCase    *booking.ListTicketsByUserUseCase
	context    context.Context

	userID string
}

func (s *listTicketsByUserUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockTicketListerByUser(s.controller)
	s.useCase = booking.NewListTicketsByUserUseCase(s.tickets)
	s.context = context.Background()
	s.userID = "11111111-1111-1111-1111-111111111111"
}

func (s *listTicketsByUserUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listTicketsByUserUseCaseSuite) TestExecute_ReturnsWhateverTheRepositoryGives() {
	replyID := "msg-1"
	want := []entity.TicketSummary{
		{SupportTicket: entity.SupportTicket{ID: "t1", CreatedBy: s.userID}, LastAdminReplyID: &replyID},
		{SupportTicket: entity.SupportTicket{ID: "t2", CreatedBy: s.userID}},
	}
	s.tickets.EXPECT().ListTicketsByUser(gomock.Any(), s.userID).Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.userID)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listTicketsByUserUseCaseSuite) TestExecute_PropagatesRepositoryError() {
	s.tickets.EXPECT().ListTicketsByUser(gomock.Any(), s.userID).
		Return(nil, errors.NewInternalError("booking.ListTicketsByUser", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.userID)

	s.Nil(got)
	s.Error(err)
}
