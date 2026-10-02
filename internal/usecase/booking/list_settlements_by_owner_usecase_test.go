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

func TestListSettlementsByOwnerUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listSettlementsByOwnerUseCaseSuite))
}

type listSettlementsByOwnerUseCaseSuite struct {
	suite.Suite
	controller  *gomock.Controller
	settlements *mocks.MockOwnerSettlementLister
	useCase     *booking.ListSettlementsByOwnerUseCase
	context     context.Context

	ownerID string
}

func (s *listSettlementsByOwnerUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settlements = mocks.NewMockOwnerSettlementLister(s.controller)
	s.useCase = booking.NewListSettlementsByOwnerUseCase(s.settlements)
	s.context = context.Background()
	s.ownerID = "11111111-1111-1111-1111-111111111111"
}

func (s *listSettlementsByOwnerUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

// TestExecute_ReturnsAllStatusesUnfiltered proves the self-service list surfaces both a
// still-PENDING draft and a confirmed PAID settlement to their own owner -- it passes no status
// filter to the repository, so the owner can see "being prepared" as well as "paid".
func (s *listSettlementsByOwnerUseCaseSuite) TestExecute_ReturnsAllStatusesUnfiltered() {
	want := []entity.Settlement{
		{ID: "s1", OwnerID: s.ownerID, Status: enum.SettlementStatusPending},
		{ID: "s2", OwnerID: s.ownerID, Status: enum.SettlementStatusPaid},
	}
	s.settlements.EXPECT().ListSettlementsByOwner(gomock.Any(), s.ownerID, (*enum.SettlementStatus)(nil)).Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Require().NoError(err)
	s.Equal(want, got)
}
