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

func TestMarkPayoutPaidUseCaseSuite(t *testing.T) {
	suite.Run(t, new(markPayoutPaidUseCaseSuite))
}

type markPayoutPaidUseCaseSuite struct {
	suite.Suite
	controller   *gomock.Controller
	payoutGetter *mocks.MockPayoutByIDGetter
	payouts      *mocks.MockPayoutMarker
	bookings     *mocks.MockBookingGetter
	spots        *mocks.MockSpotDetailsLookup
	storage      *mocks.MockStorageService
	events       *mocks.MockEventPublisher
	useCase      *booking.MarkPayoutPaidUseCase
	context      context.Context

	payoutID  string
	bookingID string
	spotID    string
	ownerID   string
	paymentID string
	adminID   string
}

func (s *markPayoutPaidUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.payoutGetter = mocks.NewMockPayoutByIDGetter(s.controller)
	s.payouts = mocks.NewMockPayoutMarker(s.controller)
	s.bookings = mocks.NewMockBookingGetter(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.storage = mocks.NewMockStorageService(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewMarkPayoutPaidUseCase(s.payoutGetter, s.payouts, s.bookings, s.spots, s.storage, s.events)
	s.context = context.Background()

	s.payoutID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.bookingID = "11111111-1111-1111-1111-111111111111"
	s.spotID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
	s.paymentID = "mp-payment-98765"
	s.adminID = "44444444-4444-4444-4444-444444444444"
}

func (s *markPayoutPaidUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *markPayoutPaidUseCaseSuite) pendingPayout() entity.Payout {
	return entity.Payout{
		ID:        s.payoutID,
		BookingID: s.bookingID,
		OwnerID:   s.ownerID,
		PaymentID: s.paymentID,
		NetAmount: 9500,
		Status:    enum.PayoutStatusPending,
	}
}

func (s *markPayoutPaidUseCaseSuite) paidPayout() entity.Payout {
	return entity.Payout{
		ID:        s.payoutID,
		BookingID: s.bookingID,
		OwnerID:   s.ownerID,
		PaymentID: s.paymentID,
		NetAmount: 9500,
		Status:    enum.PayoutStatusPaid,
	}
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_NoProof_MarksPaidAndPublishes() {
	s.payouts.EXPECT().MarkPayoutPaid(gomock.Any(), s.payoutID, s.adminID, (*string)(nil)).
		Return(s.paidPayout(), nil)
	s.bookings.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{ID: s.bookingID, ParkingSpotID: s.spotID}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera en Palermo"}, nil)

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

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, nil, "")

	s.Require().NoError(err)
	s.Equal(s.paidPayout(), got)
	s.Equal("payout.paid", gotKey)
	s.Equal(s.ownerID, gotPayload["recipient_user_id"])
	s.Equal(float64(9500), gotPayload["net_amount"])
	s.Equal("Cochera en Palermo", gotPayload["parking_spot_title"])
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_WithProof_UploadsThenMarksPaid() {
	proofData := []byte("fake-image-bytes")
	proofURL := "https://cdn.example.com/payout-proofs/" + s.ownerID + "/" + s.paymentID + ".jpg"

	s.payoutGetter.EXPECT().GetPayoutByID(gomock.Any(), s.payoutID).Return(s.pendingPayout(), nil)
	s.storage.EXPECT().UploadPayoutProof(gomock.Any(), s.ownerID, s.paymentID, "image/jpeg", proofData).
		Return(proofURL, nil)
	s.payouts.EXPECT().MarkPayoutPaid(gomock.Any(), s.payoutID, s.adminID, &proofURL).
		Return(s.paidPayout(), nil)
	s.bookings.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{ID: s.bookingID, ParkingSpotID: s.spotID}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, proofData, "image/jpeg")

	s.Require().NoError(err)
	s.Equal(s.paidPayout(), got)
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_AlreadyPaid_RejectsWithConflict() {
	s.payouts.EXPECT().MarkPayoutPaid(gomock.Any(), s.payoutID, s.adminID, (*string)(nil)).
		Return(entity.Payout{}, errors.NewConflict("este pago ya fue marcado como pagado"))

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, nil, "")

	s.Equal(entity.Payout{}, got)
	s.True(errors.IsConflict(err))
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_ProofUploadFails_AbortsBeforeMarkingPaid() {
	proofData := []byte("fake-image-bytes")
	s.payoutGetter.EXPECT().GetPayoutByID(gomock.Any(), s.payoutID).Return(s.pendingPayout(), nil)
	s.storage.EXPECT().UploadPayoutProof(gomock.Any(), s.ownerID, s.paymentID, "image/png", proofData).
		Return("", errors.NewInternalError("storage.UploadPayoutProof", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, proofData, "image/png")

	s.Equal(entity.Payout{}, got)
	s.Error(err)
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_ProofSupplied_PayoutLookupFails_AbortsBeforeUploading() {
	proofData := []byte("fake-image-bytes")
	s.payoutGetter.EXPECT().GetPayoutByID(gomock.Any(), s.payoutID).
		Return(entity.Payout{}, errors.NewNotFound("payout", s.payoutID))

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, proofData, "image/png")

	s.Equal(entity.Payout{}, got)
	s.True(errors.IsNotFound(err))
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_PublishFails_StillReturnsMarkedPaidPayout() {
	s.payouts.EXPECT().MarkPayoutPaid(gomock.Any(), s.payoutID, s.adminID, (*string)(nil)).
		Return(s.paidPayout(), nil)
	s.bookings.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{ID: s.bookingID, ParkingSpotID: s.spotID}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.NewInternalError("publish failed", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, nil, "")

	s.Require().NoError(err)
	s.Equal(s.paidPayout(), got)
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_SpotLookupFails_StillReturnsMarkedPaidPayout() {
	s.payouts.EXPECT().MarkPayoutPaid(gomock.Any(), s.payoutID, s.adminID, (*string)(nil)).
		Return(s.paidPayout(), nil)
	s.bookings.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{ID: s.bookingID, ParkingSpotID: s.spotID}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{}, errors.NewUpstream("parkingclient.GetSpotDetails", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, nil, "")

	s.Require().NoError(err)
	s.Equal(s.paidPayout(), got)
}

func (s *markPayoutPaidUseCaseSuite) TestExecute_BookingLookupFails_StillReturnsMarkedPaidPayout() {
	s.payouts.EXPECT().MarkPayoutPaid(gomock.Any(), s.payoutID, s.adminID, (*string)(nil)).
		Return(s.paidPayout(), nil)
	s.bookings.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	got, err := s.useCase.Execute(s.context, s.payoutID, s.adminID, nil, "")

	s.Require().NoError(err)
	s.Equal(s.paidPayout(), got)
}
