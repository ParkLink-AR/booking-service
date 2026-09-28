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

func TestDraftSettlementUseCaseSuite(t *testing.T) {
	suite.Run(t, new(draftSettlementUseCaseSuite))
}

type draftSettlementUseCaseSuite struct {
	suite.Suite
	controller  *gomock.Controller
	settlements *mocks.MockSettlementDrafter
	useCase     *booking.DraftSettlementUseCase
	context     context.Context

	ownerID string
	adminID string
}

func (s *draftSettlementUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settlements = mocks.NewMockSettlementDrafter(s.controller)
	s.useCase = booking.NewDraftSettlementUseCase(s.settlements)
	s.context = context.Background()

	s.ownerID = "11111111-1111-1111-1111-111111111111"
	s.adminID = "22222222-2222-2222-2222-222222222222"
}

func (s *draftSettlementUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *draftSettlementUseCaseSuite) TestExecute_SweepsUpPendingPayouts_ReturnsDraftAndPayouts() {
	want := entity.Settlement{ID: "settlement-1", OwnerID: s.ownerID, NetAmount: 14000}
	wantPayouts := []entity.Payout{{ID: "p1", NetAmount: 9500}, {ID: "p2", NetAmount: 4500}}
	s.settlements.EXPECT().CreateSettlementDraft(gomock.Any(), s.ownerID, s.adminID).Return(want, wantPayouts, nil)

	got, gotPayouts, err := s.useCase.Execute(s.context, s.ownerID, s.adminID)

	s.Require().NoError(err)
	s.Equal(want, got)
	s.Equal(wantPayouts, gotPayouts)
}

func (s *draftSettlementUseCaseSuite) TestExecute_NoPendingPayouts_PropagatesNothingToSettle() {
	s.settlements.EXPECT().CreateSettlementDraft(gomock.Any(), s.ownerID, s.adminID).
		Return(entity.Settlement{}, nil, errors.NewInvalidInput("NOTHING_TO_SETTLE", "el propietario no tiene pagos pendientes"))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.ownerID, s.adminID)

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.True(errors.IsInvalidInput(err))
	s.Equal("NOTHING_TO_SETTLE", errors.InvalidInputCode(err))
}
