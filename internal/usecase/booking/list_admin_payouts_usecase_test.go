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
	proofURLs  *mocks.MockProofURLResolver
	useCase    *booking.ListAdminPayoutsUseCase
	context    context.Context
}

func (s *listAdminPayoutsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.payouts = mocks.NewMockPayoutLister(s.controller)
	s.proofURLs = mocks.NewMockProofURLResolver(s.controller)
	s.useCase = booking.NewListAdminPayoutsUseCase(s.payouts, s.proofURLs)
	s.context = context.Background()
}

func (s *listAdminPayoutsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listAdminPayoutsUseCaseSuite) TestExecute_NoFilter_ReturnsWhateverTheRepositoryReturns() {
	want := []entity.Payout{{ID: "p1"}, {ID: "p2"}}
	s.payouts.EXPECT().ListPayouts(gomock.Any(), (*enum.PayoutStatus)(nil), (*string)(nil)).Return(want, nil)

	got, err := s.useCase.Execute(s.context, nil, nil)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listAdminPayoutsUseCaseSuite) TestExecute_WithStatusFilter_PassesItThrough() {
	status := enum.PayoutStatusPaid
	want := []entity.Payout{{ID: "p1", Status: enum.PayoutStatusPaid}}
	s.payouts.EXPECT().ListPayouts(gomock.Any(), &status, (*string)(nil)).Return(want, nil)

	got, err := s.useCase.Execute(s.context, &status, nil)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listAdminPayoutsUseCaseSuite) TestExecute_WithOwnerIDFilter_PassesItThrough() {
	ownerID := "owner-1"
	want := []entity.Payout{{ID: "p1", OwnerID: ownerID}}
	s.payouts.EXPECT().ListPayouts(gomock.Any(), (*enum.PayoutStatus)(nil), &ownerID).Return(want, nil)

	got, err := s.useCase.Execute(s.context, nil, &ownerID)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listAdminPayoutsUseCaseSuite) TestExecute_ResolvesProofURLForEachPayout() {
	proofKey := "payout-proofs/owner1/settlement1.jpg"
	presignedURL := "https://r2.example.com/signed?sig=abc"
	want := []entity.Payout{{ID: "p1", ProofURL: &proofKey}, {ID: "p2"}}
	s.payouts.EXPECT().ListPayouts(gomock.Any(), (*enum.PayoutStatus)(nil), (*string)(nil)).Return(want, nil)
	s.proofURLs.EXPECT().ResolveProofURL(gomock.Any(), proofKey).Return(presignedURL, nil)

	got, err := s.useCase.Execute(s.context, nil, nil)

	s.Require().NoError(err)
	s.Require().NotNil(got[0].ProofURL)
	s.Equal(presignedURL, *got[0].ProofURL)
	s.Nil(got[1].ProofURL)
}
