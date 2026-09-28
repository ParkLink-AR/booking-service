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

func TestGetSettlementUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getSettlementUseCaseSuite))
}

type getSettlementUseCaseSuite struct {
	suite.Suite
	controller  *gomock.Controller
	settlements *mocks.MockSettlementGetter
	payouts     *mocks.MockPayoutsBySettlementLister
	useCase     *booking.GetSettlementUseCase
	context     context.Context

	settlementID string
}

func (s *getSettlementUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settlements = mocks.NewMockSettlementGetter(s.controller)
	s.payouts = mocks.NewMockPayoutsBySettlementLister(s.controller)
	s.useCase = booking.NewGetSettlementUseCase(s.settlements, s.payouts)
	s.context = context.Background()
	s.settlementID = "11111111-1111-1111-1111-111111111111"
}

func (s *getSettlementUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getSettlementUseCaseSuite) TestExecute_Found_ReturnsSettlementAndPayouts() {
	want := entity.Settlement{ID: s.settlementID}
	wantPayouts := []entity.Payout{{ID: "p1"}}
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).Return(want, nil)
	s.payouts.EXPECT().ListPayoutsBySettlement(gomock.Any(), s.settlementID).Return(wantPayouts, nil)

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID)

	s.Require().NoError(err)
	s.Equal(want, got)
	s.Equal(wantPayouts, gotPayouts)
}

func (s *getSettlementUseCaseSuite) TestExecute_PayoutsLookupFails_PropagatesError() {
	want := entity.Settlement{ID: s.settlementID}
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).Return(want, nil)
	s.payouts.EXPECT().ListPayoutsBySettlement(gomock.Any(), s.settlementID).
		Return(nil, errors.NewInternalError("booking.ListPayoutsBySettlement", context.DeadlineExceeded))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID)

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.Error(err)
}

func (s *getSettlementUseCaseSuite) TestExecute_NotFound_PropagatesError() {
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).
		Return(entity.Settlement{}, errors.NewNotFound("settlement", s.settlementID))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID)

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.True(errors.IsNotFound(err))
}
