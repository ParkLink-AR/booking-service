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

func TestGetOwnSettlementUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getOwnSettlementUseCaseSuite))
}

type getOwnSettlementUseCaseSuite struct {
	suite.Suite
	controller  *gomock.Controller
	settlements *mocks.MockSettlementGetter
	payouts     *mocks.MockPayoutsBySettlementLister
	useCase     *booking.GetOwnSettlementUseCase
	context     context.Context

	settlementID string
	ownerID      string
}

func (s *getOwnSettlementUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settlements = mocks.NewMockSettlementGetter(s.controller)
	s.payouts = mocks.NewMockPayoutsBySettlementLister(s.controller)
	s.useCase = booking.NewGetOwnSettlementUseCase(s.settlements, s.payouts)
	s.context = context.Background()
	s.settlementID = "11111111-1111-1111-1111-111111111111"
	s.ownerID = "22222222-2222-2222-2222-222222222222"
}

func (s *getOwnSettlementUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getOwnSettlementUseCaseSuite) TestExecute_OwnerAndPaid_ReturnsSettlementAndPayouts() {
	want := entity.Settlement{ID: s.settlementID, OwnerID: s.ownerID, Status: enum.SettlementStatusPaid}
	wantPayouts := []entity.Payout{{ID: "p1"}}
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).Return(want, nil)
	s.payouts.EXPECT().ListPayoutsBySettlement(gomock.Any(), s.settlementID).Return(wantPayouts, nil)

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.ownerID)

	s.Require().NoError(err)
	s.Equal(want, got)
	s.Equal(wantPayouts, gotPayouts)
}

func (s *getOwnSettlementUseCaseSuite) TestExecute_NotTheOwner_RejectsWithForbidden() {
	want := entity.Settlement{ID: s.settlementID, OwnerID: s.ownerID, Status: enum.SettlementStatusPaid}
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).Return(want, nil)

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, "someone-else")

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.True(errors.IsForbidden(err))
}

// A still-PENDING draft (admin already prepared it, hasn't confirmed payment yet) is visible
// to its own owner too, same as a PAID one -- the owner sees "being prepared" in the meantime.
func (s *getOwnSettlementUseCaseSuite) TestExecute_StillPending_VisibleToOwner() {
	want := entity.Settlement{ID: s.settlementID, OwnerID: s.ownerID, Status: enum.SettlementStatusPending}
	wantPayouts := []entity.Payout{{ID: "p1"}}
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).Return(want, nil)
	s.payouts.EXPECT().ListPayoutsBySettlement(gomock.Any(), s.settlementID).Return(wantPayouts, nil)

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.ownerID)

	s.Require().NoError(err)
	s.Equal(want, got)
	s.Equal(wantPayouts, gotPayouts)
}

func (s *getOwnSettlementUseCaseSuite) TestExecute_PayoutsLookupFails_PropagatesError() {
	want := entity.Settlement{ID: s.settlementID, OwnerID: s.ownerID, Status: enum.SettlementStatusPaid}
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).Return(want, nil)
	s.payouts.EXPECT().ListPayoutsBySettlement(gomock.Any(), s.settlementID).
		Return(nil, errors.NewInternalError("booking.ListPayoutsBySettlement", context.DeadlineExceeded))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.ownerID)

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.Error(err)
}

func (s *getOwnSettlementUseCaseSuite) TestExecute_SettlementLookupFails_PropagatesError() {
	s.settlements.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).
		Return(entity.Settlement{}, errors.NewNotFound("settlement", s.settlementID))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.ownerID)

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.True(errors.IsNotFound(err))
}
