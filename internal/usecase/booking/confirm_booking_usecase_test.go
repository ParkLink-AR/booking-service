package booking_test

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestConfirmBookingUseCaseSuite(t *testing.T) {
	suite.Run(t, new(confirmBookingUseCaseSuite))
}

type confirmBookingUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	confirmer  *mocks.MockBookingConfirmer
	spots      *mocks.MockSpotLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.ConfirmBookingUseCase
	context    context.Context

	bookingID string
	paymentID string
	amount    int
	spotID    string
	driverID  string
	ownerID   string
}

func (s *confirmBookingUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.confirmer = mocks.NewMockBookingConfirmer(s.controller)
	s.spots = mocks.NewMockSpotLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewConfirmBookingUseCase(s.confirmer, s.spots, s.events)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.paymentID = "pay-123"
	s.amount = 150000
	s.spotID = "11111111-1111-1111-1111-111111111111"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *confirmBookingUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *confirmBookingUseCaseSuite) pendingBooking() entity.Booking {
	holdExpiresAt := time.Now().Add(5 * time.Minute)
	return entity.Booking{
		ID: s.bookingID, ParkingSpotID: s.spotID, UserID: s.driverID,
		Status: enum.BookingStatusPendingPayment, HoldExpiresAt: &holdExpiresAt, Amount: s.amount,
		// Starts in three days: the owner gets the full approval window.
		Start: time.Now().Add(72 * time.Hour), End: time.Now().Add(74 * time.Hour),
	}
}

// captureEvent records the single Publish call the use case is expected to make.
func (s *confirmBookingUseCaseSuite) captureEvent(gotKey *string, gotPayload *map[string]any) *gomock.Call {
	return s.events.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, routingKey string, payload any) error {
			*gotKey = routingKey
			raw, err := json.Marshal(payload)
			s.Require().NoError(err)
			s.Require().NoError(json.Unmarshal(raw, gotPayload))
			return nil
		})
}

func (s *confirmBookingUseCaseSuite) TestExecute_GoesToPendingApprovalAndPublishes() {
	confirmed := s.pendingBooking()
	confirmed.Status = enum.BookingStatusPendingOwnerApproval

	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera en Palermo"}, nil)

	s.confirmer.EXPECT().
		ConfirmBooking(gomock.Any(), s.bookingID, s.paymentID, enum.BookingStatusPendingOwnerApproval, gomock.Any(), gomock.Nil()).
		DoAndReturn(func(_ context.Context, _, _ string, _ enum.BookingStatus, deadline *time.Time, verificationCode *string) (entity.Booking, error) {
			s.Require().NotNil(deadline)
			s.WithinDuration(time.Now().Add(24*time.Hour), *deadline, time.Minute)
			s.Nil(verificationCode, "no code should be generated before the owner approves")
			return confirmed, nil
		})

	var gotKey string
	var gotPayload map[string]any
	s.captureEvent(&gotKey, &gotPayload)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.Require().NoError(err)
	s.Equal(confirmed, got)
	s.Equal("booking.pending_approval", gotKey)
	s.Equal("booking.pending_approval", gotPayload["event_type"])
	s.Equal(s.ownerID, gotPayload["owner_user_id"])
	s.Equal(s.driverID, gotPayload["driver_user_id"])
	s.Equal("Cochera en Palermo", gotPayload["parking_spot_title"])
	s.Equal(s.bookingID, gotPayload["booking_id"])
	s.NotEmpty(gotPayload["event_id"])
}

func (s *confirmBookingUseCaseSuite) TestExecute_PublishFailureIsSwallowed() {
	confirmed := s.pendingBooking()
	confirmed.Status = enum.BookingStatusPendingOwnerApproval

	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "x"}, nil)
	s.confirmer.EXPECT().
		ConfirmBooking(gomock.Any(), s.bookingID, s.paymentID, enum.BookingStatusPendingOwnerApproval, gomock.Any(), gomock.Nil()).
		Return(confirmed, nil)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewUpstream("rabbitmq", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.Require().NoError(err)
	s.Equal(confirmed, got)
}

func (s *confirmBookingUseCaseSuite) TestExecute_SpotLookupError_NoConfirmNoPublish() {
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{}, errors.NewUpstream("parkingclient", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsUpstream(err))
}

func (s *confirmBookingUseCaseSuite) assertCode(err error, code string) {
	var invalid *errors.InvalidInputError
	s.Require().True(stderrors.As(err, &invalid), "expected InvalidInputError, got %v", err)
	s.Equal(code, invalid.Code)
}

func (s *confirmBookingUseCaseSuite) TestExecute_SamePaymentAgain_IsIdempotent() {
	paidAt := time.Now()
	confirmed := s.pendingBooking()
	confirmed.Status = enum.BookingStatusPendingOwnerApproval
	confirmed.PaymentID = s.paymentID
	confirmed.PaidAt = &paidAt
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(confirmed, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.Require().NoError(err)
	s.Equal(confirmed, got)
}

func (s *confirmBookingUseCaseSuite) TestExecute_HoldElapsed_RejectedAsHoldExpired() {
	late := s.pendingBooking()
	elapsed := time.Now().Add(-time.Minute)
	late.HoldExpiresAt = &elapsed
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(late, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.assertCode(err, booking.ConfirmErrorHoldExpired)
}

func (s *confirmBookingUseCaseSuite) TestExecute_ExpiredBooking_RejectedAsHoldExpired() {
	expired := s.pendingBooking()
	expired.Status = enum.BookingStatusExpired
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(expired, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.assertCode(err, booking.ConfirmErrorHoldExpired)
}

func (s *confirmBookingUseCaseSuite) TestExecute_WrongAmount_RejectedAsAmountMismatch() {
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, 100)

	s.assertCode(err, booking.ConfirmErrorAmountMismatch)
}

// The payment of a booking that took on the driver's pending balances covers both.
func (s *confirmBookingUseCaseSuite) TestExecute_BookingWithDebt_ExpectsThePriceAndTheDebt() {
	withDebt := s.pendingBooking()
	withDebt.DebtAmount = 6000
	confirmed := withDebt
	confirmed.Status = enum.BookingStatusPendingOwnerApproval
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(withDebt, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera"}, nil)
	s.confirmer.EXPECT().ConfirmBooking(gomock.Any(), s.bookingID, s.paymentID, enum.BookingStatusPendingOwnerApproval, gomock.Any(), gomock.Nil()).Return(confirmed, nil)
	var gotKey string
	var gotPayload map[string]any
	s.captureEvent(&gotKey, &gotPayload)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount+6000)

	s.Require().NoError(err)
	s.Equal("booking.pending_approval", gotKey)
}

func (s *confirmBookingUseCaseSuite) TestExecute_BookingWithDebt_PayingOnlyThePriceIsAMismatch() {
	withDebt := s.pendingBooking()
	withDebt.DebtAmount = 6000
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(withDebt, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.assertCode(err, booking.ConfirmErrorAmountMismatch)
}

func (s *confirmBookingUseCaseSuite) TestExecute_PaidWithAnotherPayment_RejectedAsNotPayable() {
	paidAt := time.Now()
	other := s.pendingBooking()
	other.Status = enum.BookingStatusPendingOwnerApproval
	other.PaymentID = "pay-other"
	other.PaidAt = &paidAt
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(other, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.assertCode(err, booking.ConfirmErrorNotPayable)
}

func (s *confirmBookingUseCaseSuite) TestExecute_LostRaceToSamePayment_ReturnsConfirmed() {
	paidAt := time.Now()
	confirmed := s.pendingBooking()
	confirmed.Status = enum.BookingStatusPendingOwnerApproval
	confirmed.PaymentID = s.paymentID
	confirmed.PaidAt = &paidAt

	gomock.InOrder(
		s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil),
		s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(confirmed, nil),
	)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).Return(entity.SpotDetails{OwnerID: s.ownerID}, nil)
	s.confirmer.EXPECT().
		ConfirmBooking(gomock.Any(), s.bookingID, s.paymentID, enum.BookingStatusPendingOwnerApproval, gomock.Any(), gomock.Nil()).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.Require().NoError(err)
	s.Equal(confirmed, got)
}

// An extension continues a stay the owner already accepted: paying it confirms it outright,
// carries over the arrival check, and tells both sides it was extended.
func (s *confirmBookingUseCaseSuite) TestExecute_Extension_ConfirmsDirectlyAndPublishesExtended() {
	parentID := "44444444-4444-4444-4444-444444444444"
	pending := s.pendingBooking()
	pending.ParentBookingID = &parentID
	confirmed := pending
	confirmed.Status = enum.BookingStatusConfirmed
	confirmed.ParentBookingID = nil

	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(pending, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera"}, nil)
	s.confirmer.EXPECT().
		ConfirmBooking(gomock.Any(), s.bookingID, s.paymentID, enum.BookingStatusConfirmed, gomock.Nil(), gomock.Nil()).
		Return(confirmed, nil)
	s.confirmer.EXPECT().PropagateVerificationToExtensions(gomock.Any(), parentID).Return(nil)

	var gotKey string
	var gotPayload map[string]any
	s.captureEvent(&gotKey, &gotPayload)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID, s.amount)

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusConfirmed, got.Status)
	s.Equal("booking.extended", gotKey)
	s.Equal(parentID, gotPayload["parent_booking_id"])
}

type ownerDeadlineCase struct {
	name     string
	startsIn time.Duration
	want     time.Duration
}

var ownerDeadlineCases = []ownerDeadlineCase{
	{"a booking days away gets the whole window", 72 * time.Hour, 24 * time.Hour},
	{"a booking in five hours: until it starts", 5 * time.Hour, 5 * time.Hour},
	{"a last-minute booking still gets 15 minutes", 5 * time.Minute, 15 * time.Minute},
	{"one already under way (a stay booked from now) gets 15 minutes", -time.Minute, 15 * time.Minute},
}

func (s *confirmBookingUseCaseSuite) TestOwnerResponseDeadline() {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	for _, tc := range ownerDeadlineCases {
		got := booking.OwnerResponseDeadline(now, now.Add(tc.startsIn))
		s.Equal(now.Add(tc.want), got, tc.name)
	}
}
