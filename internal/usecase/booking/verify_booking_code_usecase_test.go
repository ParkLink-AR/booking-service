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

func TestVerifyBookingCodeUseCaseSuite(t *testing.T) {
	suite.Run(t, new(verifyBookingCodeUseCaseSuite))
}

type verifyBookingCodeUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	verifier   *mocks.MockBookingCodeVerifier
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.VerifyBookingCodeUseCase
	context    context.Context

	bookingID string
	spotID    string
	driverID  string
	ownerID   string
	code      string
}

func (s *verifyBookingCodeUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.verifier = mocks.NewMockBookingCodeVerifier(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewVerifyBookingCodeUseCase(s.verifier, s.spots, s.events)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.spotID = "11111111-1111-1111-1111-111111111111"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
	s.code = "9542"
}

func (s *verifyBookingCodeUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *verifyBookingCodeUseCaseSuite) confirmedBooking() entity.Booking {
	code := s.code
	return entity.Booking{
		ID:               s.bookingID,
		ParkingSpotID:    s.spotID,
		UserID:           s.driverID,
		Status:           enum.BookingStatusConfirmed,
		VerificationCode: &code,
		// Inside the verification window: started 10 minutes ago, ends in an hour.
		Start: time.Now().Add(-10 * time.Minute),
		End:   time.Now().Add(time.Hour),
	}
}

func (s *verifyBookingCodeUseCaseSuite) TestExecute_BeforeWindowOpens_Rejected() {
	early := s.confirmedBooking()
	early.Start = time.Now().Add(2 * time.Hour)
	early.End = early.Start.Add(time.Hour)

	s.verifier.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(early, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID}, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, s.code)

	var invalid *errors.InvalidInputError
	s.Require().True(stderrors.As(err, &invalid))
	s.Equal("VERIFICATION_NOT_OPEN_YET", invalid.Code)
}

func (s *verifyBookingCodeUseCaseSuite) TestExecute_WithinThirtyMinutesBeforeStart_Allowed() {
	soon := s.confirmedBooking()
	soon.Start = time.Now().Add(20 * time.Minute)
	soon.End = soon.Start.Add(time.Hour)
	verified := soon
	verified.Status = enum.BookingStatusInProgress

	s.verifier.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(soon, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID}, nil)
	s.verifier.EXPECT().VerifyBookingCode(gomock.Any(), s.bookingID, s.code).Return(verified, nil)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, s.code)

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusInProgress, got.Status)
}

func (s *verifyBookingCodeUseCaseSuite) TestExecute_AfterBookingEnded_Rejected() {
	ended := s.confirmedBooking()
	ended.Start = time.Now().Add(-2 * time.Hour)
	ended.End = time.Now().Add(-time.Minute)

	s.verifier.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(ended, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID}, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, s.code)

	var invalid *errors.InvalidInputError
	s.Require().True(stderrors.As(err, &invalid))
	s.Equal("VERIFICATION_WINDOW_CLOSED", invalid.Code)
}

func (s *verifyBookingCodeUseCaseSuite) TestExecute_OwnerEntersCorrectCode_TransitionsAndPublishesToDriver() {
	verified := s.confirmedBooking()
	verified.Status = enum.BookingStatusInProgress

	s.verifier.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.confirmedBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera en Palermo"}, nil)
	s.verifier.EXPECT().VerifyBookingCode(gomock.Any(), s.bookingID, s.code).Return(verified, nil)

	var gotKey string
	var gotPayload map[string]any
	s.events.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, routingKey string, payload any) error {
			gotKey = routingKey
			raw, err := json.Marshal(payload)
			s.Require().NoError(err)
			s.Require().NoError(json.Unmarshal(raw, &gotPayload))
			return nil
		})

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, s.code)

	s.Require().NoError(err)
	s.Equal(verified, got)
	s.Equal("booking.verified", gotKey)
	s.Equal("booking.verified", gotPayload["event_type"])
	s.Equal(s.driverID, gotPayload["driver_user_id"])
	s.Equal(s.ownerID, gotPayload["owner_user_id"])
}

func (s *verifyBookingCodeUseCaseSuite) TestExecute_CallerIsNotOwner_ForbiddenNoVerifyNoPublish() {
	s.verifier.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.confirmedBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "x"}, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, "44444444-4444-4444-4444-444444444444", s.code)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsForbidden(err))
}

func (s *verifyBookingCodeUseCaseSuite) TestExecute_WrongCode_InvalidInputNoPublish() {
	s.verifier.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.confirmedBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "x"}, nil)
	s.verifier.EXPECT().VerifyBookingCode(gomock.Any(), s.bookingID, "0000").
		Return(entity.Booking{}, errors.NewInvalidInput("INVALID_CODE", "código incorrecto"))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, "0000")

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
}

func (s *verifyBookingCodeUseCaseSuite) TestExecute_TooManyAttempts_ConflictNoPublish() {
	s.verifier.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.confirmedBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "x"}, nil)
	s.verifier.EXPECT().VerifyBookingCode(gomock.Any(), s.bookingID, s.code).
		Return(entity.Booking{}, errors.NewConflict("se alcanzó el máximo de intentos para este código"))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, s.code)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsConflict(err))
}
