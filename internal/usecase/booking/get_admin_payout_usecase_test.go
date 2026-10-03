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

func TestGetAdminPayoutUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getAdminPayoutUseCaseSuite))
}

type getAdminPayoutUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	payouts    *mocks.MockPayoutGetter
	proofURLs  *mocks.MockProofURLResolver
	useCase    *booking.GetAdminPayoutUseCase
	context    context.Context
}

func (s *getAdminPayoutUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.payouts = mocks.NewMockPayoutGetter(s.controller)
	s.proofURLs = mocks.NewMockProofURLResolver(s.controller)
	s.useCase = booking.NewGetAdminPayoutUseCase(s.payouts, s.proofURLs)
	s.context = context.Background()
}

func (s *getAdminPayoutUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getAdminPayoutUseCaseSuite) TestExecute_Found_ReturnsPayout() {
	want := entity.Payout{ID: "p1"}
	s.payouts.EXPECT().GetPayoutByID(gomock.Any(), "p1").Return(want, nil)

	got, err := s.useCase.Execute(s.context, "p1")

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *getAdminPayoutUseCaseSuite) TestExecute_WithProof_ResolvesProofURL() {
	proofKey := "payout-proofs/owner1/settlement1.jpg"
	presignedURL := "https://r2.example.com/signed?sig=abc"
	want := entity.Payout{ID: "p1", ProofURL: &proofKey}
	s.payouts.EXPECT().GetPayoutByID(gomock.Any(), "p1").Return(want, nil)
	s.proofURLs.EXPECT().ResolveProofURL(gomock.Any(), proofKey).Return(presignedURL, nil)

	got, err := s.useCase.Execute(s.context, "p1")

	s.Require().NoError(err)
	s.Require().NotNil(got.ProofURL)
	s.Equal(presignedURL, *got.ProofURL)
}

func (s *getAdminPayoutUseCaseSuite) TestExecute_NotFound_PropagatesError() {
	s.payouts.EXPECT().GetPayoutByID(gomock.Any(), "missing").
		Return(entity.Payout{}, errors.NewNotFound("payout", "missing"))

	got, err := s.useCase.Execute(s.context, "missing")

	s.Equal(entity.Payout{}, got)
	s.True(errors.IsNotFound(err))
}
