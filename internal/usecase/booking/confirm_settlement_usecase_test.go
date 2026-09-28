package booking_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestConfirmSettlementUseCaseSuite(t *testing.T) {
	suite.Run(t, new(confirmSettlementUseCaseSuite))
}

type confirmSettlementUseCaseSuite struct {
	suite.Suite
	controller       *gomock.Controller
	settlementGetter *mocks.MockSettlementGetter
	settlements      *mocks.MockSettlementConfirmer
	storage          *mocks.MockSettlementStorageService
	events           *mocks.MockEventPublisher
	useCase          *booking.ConfirmSettlementUseCase
	context          context.Context

	settlementID string
	ownerID      string
	adminID      string
}

func (s *confirmSettlementUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settlementGetter = mocks.NewMockSettlementGetter(s.controller)
	s.settlements = mocks.NewMockSettlementConfirmer(s.controller)
	s.storage = mocks.NewMockSettlementStorageService(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewConfirmSettlementUseCase(s.settlementGetter, s.settlements, s.storage, s.events)
	s.context = context.Background()

	s.settlementID = "11111111-1111-1111-1111-111111111111"
	s.ownerID = "22222222-2222-2222-2222-222222222222"
	s.adminID = "33333333-3333-3333-3333-333333333333"
}

func (s *confirmSettlementUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *confirmSettlementUseCaseSuite) paidSettlement() entity.Settlement {
	return entity.Settlement{ID: s.settlementID, OwnerID: s.ownerID, NetAmount: 14000, Status: enum.SettlementStatusPaid}
}

func (s *confirmSettlementUseCaseSuite) TestExecute_NoProof_ConfirmsAndPublishes() {
	payouts := []entity.Payout{{ID: "p1"}, {ID: "p2"}}
	s.settlements.EXPECT().ConfirmSettlement(gomock.Any(), s.settlementID, s.adminID, (*string)(nil)).
		Return(s.paidSettlement(), payouts, nil)

	var gotKey string
	var gotPayload map[string]any
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, routingKey string, payload any) error {
			gotKey = routingKey
			raw, err := json.Marshal(payload)
			s.Require().NoError(err)
			s.Require().NoError(json.Unmarshal(raw, &gotPayload))
			return nil
		})

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.adminID, nil, "")

	s.Require().NoError(err)
	s.Equal(s.paidSettlement(), got)
	s.Equal(payouts, gotPayouts)
	s.Equal("settlement.paid", gotKey)
	s.Equal(s.ownerID, gotPayload["recipient_user_id"])
	s.Equal(float64(14000), gotPayload["net_amount"])
	s.Equal(float64(2), gotPayload["booking_count"])
}

func (s *confirmSettlementUseCaseSuite) TestExecute_WithProof_UploadsThenConfirms() {
	proofData := []byte("fake-image-bytes")
	proofURL := "https://cdn.example.com/payout-proofs/" + s.ownerID + "/" + s.settlementID + ".jpg"

	s.settlementGetter.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).
		Return(entity.Settlement{ID: s.settlementID, OwnerID: s.ownerID}, nil)
	s.storage.EXPECT().UploadPayoutProof(gomock.Any(), s.ownerID, s.settlementID, "image/jpeg", proofData).
		Return(proofURL, nil)
	s.settlements.EXPECT().ConfirmSettlement(gomock.Any(), s.settlementID, s.adminID, &proofURL).
		Return(s.paidSettlement(), []entity.Payout{{ID: "p1"}}, nil)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.adminID, proofData, "image/jpeg")

	s.Require().NoError(err)
	s.Equal(s.paidSettlement(), got)
	s.Len(gotPayouts, 1)
}

func (s *confirmSettlementUseCaseSuite) TestExecute_AlreadyConfirmed_RejectsWithConflict() {
	s.settlements.EXPECT().ConfirmSettlement(gomock.Any(), s.settlementID, s.adminID, (*string)(nil)).
		Return(entity.Settlement{}, nil, errors.NewConflict("esta liquidación ya fue confirmada"))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.adminID, nil, "")

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.True(errors.IsConflict(err))
}

func (s *confirmSettlementUseCaseSuite) TestExecute_ProofUploadFails_AbortsBeforeConfirming() {
	proofData := []byte("fake-image-bytes")
	s.settlementGetter.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).
		Return(entity.Settlement{ID: s.settlementID, OwnerID: s.ownerID}, nil)
	s.storage.EXPECT().UploadPayoutProof(gomock.Any(), s.ownerID, s.settlementID, "image/png", proofData).
		Return("", errors.NewInternalError("storage.UploadPayoutProof", context.DeadlineExceeded))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.adminID, proofData, "image/png")

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.Error(err)
}

func (s *confirmSettlementUseCaseSuite) TestExecute_ProofSupplied_SettlementLookupFails_AbortsBeforeUploading() {
	proofData := []byte("fake-image-bytes")
	s.settlementGetter.EXPECT().GetSettlementByID(gomock.Any(), s.settlementID).
		Return(entity.Settlement{}, errors.NewNotFound("settlement", s.settlementID))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.adminID, proofData, "image/png")

	s.Equal(entity.Settlement{}, got)
	s.Nil(gotPayouts)
	s.True(errors.IsNotFound(err))
}

func (s *confirmSettlementUseCaseSuite) TestExecute_PublishFails_StillReturnsConfirmedSettlement() {
	payouts := []entity.Payout{{ID: "p1"}}
	s.settlements.EXPECT().ConfirmSettlement(gomock.Any(), s.settlementID, s.adminID, (*string)(nil)).
		Return(s.paidSettlement(), payouts, nil)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.NewInternalError("publish failed", context.DeadlineExceeded))

	got, gotPayouts, err := s.useCase.Execute(s.context, s.settlementID, s.adminID, nil, "")

	s.Require().NoError(err)
	s.Equal(s.paidSettlement(), got)
	s.Equal(payouts, gotPayouts)
}
