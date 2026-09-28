package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestCancelSettlementUseCaseSuite(t *testing.T) {
	suite.Run(t, new(cancelSettlementUseCaseSuite))
}

type cancelSettlementUseCaseSuite struct {
	suite.Suite
	controller  *gomock.Controller
	settlements *mocks.MockSettlementCanceller
	useCase     *booking.CancelSettlementUseCase
	context     context.Context

	settlementID string
}

func (s *cancelSettlementUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settlements = mocks.NewMockSettlementCanceller(s.controller)
	s.useCase = booking.NewCancelSettlementUseCase(s.settlements)
	s.context = context.Background()
	s.settlementID = "11111111-1111-1111-1111-111111111111"
}

func (s *cancelSettlementUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *cancelSettlementUseCaseSuite) TestExecute_Success_ReleasesPayoutsBackToUnattached() {
	s.settlements.EXPECT().CancelSettlement(gomock.Any(), s.settlementID).Return(nil)

	err := s.useCase.Execute(s.context, s.settlementID)

	s.Require().NoError(err)
}

func (s *cancelSettlementUseCaseSuite) TestExecute_AlreadyConfirmed_PropagatesConflict() {
	s.settlements.EXPECT().CancelSettlement(gomock.Any(), s.settlementID).
		Return(errors.NewConflict("esta liquidación ya fue confirmada y no se puede cancelar"))

	err := s.useCase.Execute(s.context, s.settlementID)

	s.True(errors.IsConflict(err))
}

func (s *cancelSettlementUseCaseSuite) TestExecute_NotFound_PropagatesNotFound() {
	s.settlements.EXPECT().CancelSettlement(gomock.Any(), s.settlementID).
		Return(errors.NewNotFound("settlement", s.settlementID))

	err := s.useCase.Execute(s.context, s.settlementID)

	s.True(errors.IsNotFound(err))
}
