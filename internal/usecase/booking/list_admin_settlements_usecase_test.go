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

func TestListAdminSettlementsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listAdminSettlementsUseCaseSuite))
}

type listAdminSettlementsUseCaseSuite struct {
	suite.Suite
	controller  *gomock.Controller
	settlements *mocks.MockSettlementLister
	useCase     *booking.ListAdminSettlementsUseCase
	context     context.Context
}

func (s *listAdminSettlementsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settlements = mocks.NewMockSettlementLister(s.controller)
	s.useCase = booking.NewListAdminSettlementsUseCase(s.settlements)
	s.context = context.Background()
}

func (s *listAdminSettlementsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listAdminSettlementsUseCaseSuite) TestExecute_NoFilter_ReturnsWhateverTheRepositoryReturns() {
	want := []entity.Settlement{{ID: "s1"}, {ID: "s2"}}
	s.settlements.EXPECT().ListSettlements(gomock.Any(), (*enum.SettlementStatus)(nil)).Return(want, nil)

	got, err := s.useCase.Execute(s.context, nil)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listAdminSettlementsUseCaseSuite) TestExecute_WithStatusFilter_PassesItThrough() {
	status := enum.SettlementStatusPending
	want := []entity.Settlement{{ID: "s1", Status: enum.SettlementStatusPending}}
	s.settlements.EXPECT().ListSettlements(gomock.Any(), &status).Return(want, nil)

	got, err := s.useCase.Execute(s.context, &status)

	s.Require().NoError(err)
	s.Equal(want, got)
}
