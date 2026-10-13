package booking_test

import (
	"context"
	"strings"
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

func TestSendMessageUseCaseSuite(t *testing.T) {
	suite.Run(t, new(sendMessageUseCaseSuite))
}

type sendMessageUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	getter     *mocks.MockBookingGetter
	spots      *mocks.MockSpotDetailsLookup
	creator    *mocks.MockMessageCreator
	events     *mocks.MockEventPublisher
	useCase    *booking.SendMessageUseCase
	context    context.Context

	bookingID string
	driverID  string
	ownerID   string
	spot      entity.SpotDetails
}

func (s *sendMessageUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.getter = mocks.NewMockBookingGetter(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.creator = mocks.NewMockMessageCreator(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewSendMessageUseCase(s.getter, s.spots, s.creator, s.events)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
	s.spot = entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera Palermo"}
}

func (s *sendMessageUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

// booking is a paid one -- messages only open up once the payment went through.
func (s *sendMessageUseCaseSuite) booking() entity.Booking {
	paidAt := time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
	return entity.Booking{
		ID: s.bookingID, UserID: s.driverID, ParkingSpotID: "spot-1",
		Status: enum.BookingStatusConfirmed, PaymentID: "mp-1", PaidAt: &paidAt,
	}
}

func (s *sendMessageUseCaseSuite) TestExecute_UnpaidBooking_ReturnsNotPaidWithoutSending() {
	unpaid := entity.Booking{
		ID: s.bookingID, UserID: s.driverID, ParkingSpotID: "spot-1", Status: enum.BookingStatusPendingPayment,
	}
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(unpaid, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(s.spot, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "Hola")

	s.Require().Error(err)
	s.True(errors.IsInvalidInput(err))
	s.Equal("BOOKING_NOT_PAID", errors.InvalidInputCode(err))
}

func (s *sendMessageUseCaseSuite) TestExecute_ExpiredWithoutPayment_ReturnsNotPaid() {
	expired := entity.Booking{
		ID: s.bookingID, UserID: s.driverID, ParkingSpotID: "spot-1", Status: enum.BookingStatusExpired,
	}
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(expired, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(s.spot, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "Hola")

	s.Equal("BOOKING_NOT_PAID", errors.InvalidInputCode(err))
}

func (s *sendMessageUseCaseSuite) TestExecute_PaidBeforePaidAtExisted_StillSends() {
	legacy := entity.Booking{
		ID: s.bookingID, UserID: s.driverID, ParkingSpotID: "spot-1",
		Status: enum.BookingStatusCompleted, PaymentID: "CASH",
	}
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(legacy, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(s.spot, nil)
	s.creator.EXPECT().CreateMessage(gomock.Any(), gomock.Any()).Return(entity.BookingMessage{ID: "msg-1"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingMessageSent, gomock.Any()).Return(nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "Gracias")

	s.Require().NoError(err)
}

func (s *sendMessageUseCaseSuite) TestExecute_DriverSendsToOwner_CreatesAndPublishes() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(s.spot, nil)
	s.creator.EXPECT().CreateMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, m entity.BookingMessage) (entity.BookingMessage, error) {
			s.Equal(s.driverID, m.SenderID)
			s.Equal("¿A qué hora llegás?", m.Body)
			m.ID = "msg-1"
			return m, nil
		},
	)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingMessageSent, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal(s.ownerID, event["recipient_user_id"])
			s.Equal(booking.RecipientRoleOwner, event["recipient_role"])
			return nil
		},
	)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "¿A qué hora llegás?")

	s.Require().NoError(err)
	s.Equal("msg-1", got.ID)
}

func (s *sendMessageUseCaseSuite) TestExecute_OwnerSendsToDriver_CreatesAndPublishes() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(s.spot, nil)
	s.creator.EXPECT().CreateMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, m entity.BookingMessage) (entity.BookingMessage, error) {
			s.Equal(s.ownerID, m.SenderID)
			return m, nil
		},
	)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingMessageSent, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal(s.driverID, event["recipient_user_id"])
			s.Equal(booking.RecipientRoleDriver, event["recipient_role"])
			return nil
		},
	)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, "Te espero en la entrada")

	s.Require().NoError(err)
}

func (s *sendMessageUseCaseSuite) TestExecute_StrangerSends_Forbidden() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(s.spot, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, "someone-else", "hola")

	s.True(errors.IsForbidden(err))
}

func (s *sendMessageUseCaseSuite) TestExecute_EmptyBody_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "   ")

	s.True(errors.IsInvalidInput(err))
	s.Equal("EMPTY_MESSAGE", errors.InvalidInputCode(err))
}

func (s *sendMessageUseCaseSuite) TestExecute_TooLong_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, strings.Repeat("a", 2001))

	s.True(errors.IsInvalidInput(err))
	s.Equal("MESSAGE_TOO_LONG", errors.InvalidInputCode(err))
}

func (s *sendMessageUseCaseSuite) TestExecute_BookingNotFound_ReturnsNotFoundError() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "hola")

	s.True(errors.IsNotFound(err))
}

// A publish failure must not fail the request: the message is already saved.
func (s *sendMessageUseCaseSuite) TestExecute_PublishFailureIsSwallowed() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(s.spot, nil)
	s.creator.EXPECT().CreateMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, m entity.BookingMessage) (entity.BookingMessage, error) {
			m.ID = "msg-1"
			return m, nil
		},
	)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingMessageSent, gomock.Any()).
		Return(assertAnError)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "hola")

	s.Require().NoError(err)
	s.Equal("msg-1", got.ID)
}

var assertAnError = errors.NewInternalError("test", context.DeadlineExceeded)
