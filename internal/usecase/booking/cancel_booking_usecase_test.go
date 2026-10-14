package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestCancelBookingUseCaseSuite(t *testing.T) {
	suite.Run(t, new(cancelBookingUseCaseSuite))
}

type cancelBookingUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	canceller  *mocks.MockBookingCanceller
	refunder   *mocks.MockCancellationRefunder
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.CancelBookingUseCase
	context    context.Context

	bookingID string
	paymentID string
	driverID  string
	otherID   string
}

func (s *cancelBookingUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.canceller = mocks.NewMockBookingCanceller(s.controller)
	s.refunder = mocks.NewMockCancellationRefunder(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewCancelBookingUseCase(s.canceller, s.refunder, s.spots, s.events)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.paymentID = "pay_abc123"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.otherID = "33333333-3333-3333-3333-333333333333"
}

func (s *cancelBookingUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

// confirmedBooking builds a CONFIRMED booking whose start is far enough in the future
// that it is always within the cancellation window during tests.
func (s *cancelBookingUseCaseSuite) confirmedBooking() entity.Booking {
	start := time.Now().UTC().Add(2 * time.Hour)
	return entity.Booking{
		ID:            s.bookingID,
		ParkingSpotID: "spot-1",
		UserID:        s.driverID,
		Start:         start,
		End:           start.Add(time.Hour),
		Status:        enum.BookingStatusConfirmed,
		PaymentID:     s.paymentID,
		Amount:        1000,
	}
}

func (s *cancelBookingUseCaseSuite) TestExecute_DriverCancelsWithinWindow_CancelsAndRefunds() {
	b := s.confirmedBooking()
	cancelled := b
	cancelled.Status = enum.BookingStatusCancelled

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)
	s.canceller.EXPECT().CancelBooking(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled, 0).Return(cancelled, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: "owner-1", Title: "Cochera Palermo"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCancelled, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal(booking.CancelledByDriver, event["cancelled_by"])
			s.Equal("owner-1", event["owner_user_id"])
			s.Equal("Cochera Palermo", event["parking_spot_title"])
			return nil
		},
	)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), s.paymentID).Return(nil)
	s.canceller.EXPECT().CancelExtensions(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled).Return(nil, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusCancelled, got.Status)
}

// The owner's notice is best-effort: a spot that can't be looked up still lets the driver
// cancel and get the refund.
func (s *cancelBookingUseCaseSuite) TestExecute_SpotLookupFails_StillCancelsWithoutNotice() {
	b := s.confirmedBooking()
	cancelled := b
	cancelled.Status = enum.BookingStatusCancelled

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)
	s.canceller.EXPECT().CancelBooking(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled, 0).Return(cancelled, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{}, errors.NewNotFound("spot", "spot-1"))
	s.refunder.EXPECT().RefundPayment(gomock.Any(), s.paymentID).Return(nil)
	s.canceller.EXPECT().CancelExtensions(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled).Return(nil, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusCancelled, got.Status)
}

func (s *cancelBookingUseCaseSuite) TestExecute_OtherUserTries_ForbiddenNoCancelNoRefund() {
	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.confirmedBooking(), nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.otherID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsForbidden(err))
}

// In the booking's last hour the driver gets half back; the owner is paid the rest.
func (s *cancelBookingUseCaseSuite) TestExecute_LessThanOneHourBefore_RefundsHalf() {
	b := s.confirmedBooking()
	b.Start = time.Now().UTC().Add(30 * time.Minute)
	b.End = b.Start.Add(time.Hour)
	cancelled := b
	cancelled.Status = enum.BookingStatusCancelled

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)
	s.canceller.EXPECT().CancelBooking(gomock.Any(), s.bookingID, constants.CancellationReasonDriverLateCancelled, 500).Return(cancelled, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: "owner-1"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCancelled, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			s.EqualValues(500, toJSONMap(s.T(), payload)["refunded_amount"])
			return nil
		},
	)
	s.refunder.EXPECT().RefundPaymentAmount(gomock.Any(), s.paymentID, 500).Return(nil)
	s.canceller.EXPECT().CancelExtensions(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled).Return(nil, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Require().NoError(err)
	s.Equal(500, got.RefundedAmount)
}

// A request the owner hasn't answered is always given back in full before it starts.
func (s *cancelBookingUseCaseSuite) TestExecute_PendingApprovalInLastHour_FullRefund() {
	b := s.confirmedBooking()
	b.Status = enum.BookingStatusPendingOwnerApproval
	b.Start = time.Now().UTC().Add(20 * time.Minute)
	cancelled := b
	cancelled.Status = enum.BookingStatusCancelled

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)
	s.canceller.EXPECT().CancelBooking(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled, 0).Return(cancelled, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{}, errors.NewNotFound("spot", "spot-1"))
	s.refunder.EXPECT().RefundPayment(gomock.Any(), s.paymentID).Return(nil)
	s.canceller.EXPECT().CancelExtensions(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled).Return(nil, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Require().NoError(err)
}

func (s *cancelBookingUseCaseSuite) TestExecute_AlreadyStarted_WindowExpiredError() {
	b := s.confirmedBooking()
	b.Start = time.Now().UTC().Add(-time.Minute)

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("CANCELLATION_WINDOW_EXPIRED", errors.InvalidInputCode(err))
}

// Its paid extensions go with it, each refunded in full.
func (s *cancelBookingUseCaseSuite) TestExecute_WithExtension_CancelsAndRefundsIt() {
	b := s.confirmedBooking()
	cancelled := b
	cancelled.Status = enum.BookingStatusCancelled

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)
	s.canceller.EXPECT().CancelBooking(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled, 0).Return(cancelled, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{}, errors.NewNotFound("spot", "spot-1"))
	s.refunder.EXPECT().RefundPayment(gomock.Any(), s.paymentID).Return(nil)
	s.canceller.EXPECT().CancelExtensions(gomock.Any(), s.bookingID, constants.CancellationReasonDriverCancelled).
		Return([]entity.Booking{{ID: "ext-1", PaymentID: "pay_ext"}}, nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "pay_ext").Return(nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Require().NoError(err)
}

func (s *cancelBookingUseCaseSuite) TestExecute_BookingNotFound_ReturnsNotFoundError() {
	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsNotFound(err))
}
