package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestListAdminTicketsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listAdminTicketsUseCaseSuite))
}

type listAdminTicketsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	tickets    *mocks.MockAllTicketLister
	useCase    *booking.ListAdminTicketsUseCase
	context    context.Context
}

func (s *listAdminTicketsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.tickets = mocks.NewMockAllTicketLister(s.controller)
	s.useCase = booking.NewListAdminTicketsUseCase(s.tickets)
	s.context = context.Background()
}

func (s *listAdminTicketsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listAdminTicketsUseCaseSuite) TestExecute_NoFilter_ListsEverything() {
	want := []entity.SupportTicket{{ID: "ticket-1"}, {ID: "ticket-2"}}
	s.tickets.EXPECT().ListAllTickets(gomock.Any(), (*enum.TicketStatus)(nil)).Return(want, nil)

	got, err := s.useCase.Execute(s.context, nil)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listAdminTicketsUseCaseSuite) TestExecute_StatusFilter_PassesItThrough() {
	status := enum.TicketStatusOpen
	s.tickets.EXPECT().ListAllTickets(gomock.Any(), &status).Return(nil, nil)

	_, err := s.useCase.Execute(s.context, &status)

	s.Require().NoError(err)
}
