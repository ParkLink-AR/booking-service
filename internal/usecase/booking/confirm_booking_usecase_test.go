package booking_test

import (
	"context"
	"encoding/json"
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
	s.spotID = "11111111-1111-1111-1111-111111111111"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *confirmBookingUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *confirmBookingUseCaseSuite) pendingBooking() entity.Booking {
	return entity.Booking{ID: s.bookingID, ParkingSpotID: s.spotID, UserID: s.driverID}
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

func (s *confirmBookingUseCaseSuite) TestExecute_NonInstantSpot_GoesToPendingApprovalAndPublishes() {
	confirmed := s.pendingBooking()
	confirmed.Status = enum.BookingStatusPendingOwnerApproval

	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera en Palermo", InstantBooking: false}, nil)

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

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID)

	s.Require().NoError(err)
	s.Equal(confirmed, got)
	s.Equal("booking.pending_approval", gotKey)
	s.Equal("booking.pending_approval", gotPayload["event_type"])
	s.Equal(s.ownerID, gotPayload["owner_user_id"])
	s.Equal(s.driverID, gotPayload["driver_user_id"])
	s.Equal("Cochera en Palermo", gotPayload["parking_spot_title"])
	s.Equal(s.bookingID, gotPayload["booking_id"])
	s.NotEmpty(gotPayload["event_id"])
	_, hasAutoConfirmed := gotPayload["auto_confirmed"]
	s.False(hasAutoConfirmed, "auto_confirmed should be omitted for the approval path")
}

func (s *confirmBookingUseCaseSuite) TestExecute_InstantSpot_ConfirmsImmediatelyAndPublishes() {
	confirmed := s.pendingBooking()
	confirmed.Status = enum.BookingStatusConfirmed

	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera Centro", InstantBooking: true}, nil)
	s.confirmer.EXPECT().
		ConfirmBooking(gomock.Any(), s.bookingID, s.paymentID, enum.BookingStatusConfirmed, gomock.Nil(), gomock.Not(gomock.Nil())).
		Return(confirmed, nil)

	var gotKey string
	var gotPayload map[string]any
	s.captureEvent(&gotKey, &gotPayload)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID)

	s.Require().NoError(err)
	s.Equal(confirmed, got)
	s.Equal("booking.confirmed", gotKey)
	s.Equal("booking.confirmed", gotPayload["event_type"])
	s.Equal(true, gotPayload["auto_confirmed"])
	s.Equal(s.driverID, gotPayload["driver_user_id"])
}

func (s *confirmBookingUseCaseSuite) TestExecute_PublishFailureIsSwallowed() {
	confirmed := s.pendingBooking()
	confirmed.Status = enum.BookingStatusPendingOwnerApproval

	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "x", InstantBooking: false}, nil)
	s.confirmer.EXPECT().
		ConfirmBooking(gomock.Any(), s.bookingID, s.paymentID, enum.BookingStatusPendingOwnerApproval, gomock.Any(), gomock.Nil()).
		Return(confirmed, nil)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewUpstream("rabbitmq", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID)

	s.Require().NoError(err)
	s.Equal(confirmed, got)
}

func (s *confirmBookingUseCaseSuite) TestExecute_SpotLookupError_NoConfirmNoPublish() {
	s.confirmer.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{}, errors.NewUpstream("parkingclient", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.paymentID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsUpstream(err))
}
