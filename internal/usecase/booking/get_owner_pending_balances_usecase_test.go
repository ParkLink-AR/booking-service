package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestGetOwnerPendingBalancesUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getOwnerPendingBalancesUseCaseSuite))
}

type getOwnerPendingBalancesUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	payouts    *mocks.MockOwnerPendingBalancesGetter
	useCase    *booking.GetOwnerPendingBalancesUseCase
	context    context.Context
}

func (s *getOwnerPendingBalancesUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.payouts = mocks.NewMockOwnerPendingBalancesGetter(s.controller)
	s.useCase = booking.NewGetOwnerPendingBalancesUseCase(s.payouts)
	s.context = context.Background()
}

func (s *getOwnerPendingBalancesUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getOwnerPendingBalancesUseCaseSuite) TestExecute_ReturnsWhateverTheRepositoryReturns() {
	want := []entity.OwnerPendingBalance{
		{OwnerID: "owner-1", PendingNetAmount: 9500, PendingCount: 1, OldestPendingAt: time.Now()},
	}
	s.payouts.EXPECT().GetOwnerPendingBalances(gomock.Any()).Return(want, nil)

	got, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(want, got)
}
