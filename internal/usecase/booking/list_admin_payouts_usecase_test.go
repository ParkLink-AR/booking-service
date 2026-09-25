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

func TestListAdminPayoutsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listAdminPayoutsUseCaseSuite))
}

type listAdminPayoutsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	payouts    *mocks.MockPayoutLister
	useCase    *booking.ListAdminPayoutsUseCase
	context    context.Context
}

func (s *listAdminPayoutsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.payouts = mocks.NewMockPayoutLister(s.controller)
	s.useCase = booking.NewListAdminPayoutsUseCase(s.payouts)
	s.context = context.Background()
}

func (s *listAdminPayoutsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listAdminPayoutsUseCaseSuite) TestExecute_NoFilter_ReturnsWhateverTheRepositoryReturns() {
	want := []entity.Payout{{ID: "p1"}, {ID: "p2"}}
	s.payouts.EXPECT().ListPayouts(gomock.Any(), (*enum.PayoutStatus)(nil)).Return(want, nil)

	got, err := s.useCase.Execute(s.context, nil)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listAdminPayoutsUseCaseSuite) TestExecute_WithStatusFilter_PassesItThrough() {
	status := enum.PayoutStatusPaid
	want := []entity.Payout{{ID: "p1", Status: enum.PayoutStatusPaid}}
	s.payouts.EXPECT().ListPayouts(gomock.Any(), &status).Return(want, nil)

	got, err := s.useCase.Execute(s.context, &status)

	s.Require().NoError(err)
	s.Equal(want, got)
}
