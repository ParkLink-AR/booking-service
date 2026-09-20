package booking_test

import (
	"context"
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

func TestCancelBookingUseCaseSuite(t *testing.T) {
	suite.Run(t, new(cancelBookingUseCaseSuite))
}

type cancelBookingUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	canceller  *mocks.MockBookingCanceller
	refunder   *mocks.MockPaymentRefunder
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
	s.refunder = mocks.NewMockPaymentRefunder(s.controller)
	s.useCase = booking.NewCancelBookingUseCase(s.canceller, s.refunder)
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
		ID:        s.bookingID,
		UserID:    s.driverID,
		Start:     start,
		End:       start.Add(time.Hour),
		Status:    enum.BookingStatusConfirmed,
		PaymentID: s.paymentID,
		Amount:    1000,
	}
}

func (s *cancelBookingUseCaseSuite) TestExecute_DriverCancelsWithinWindow_CancelsAndRefunds() {
	b := s.confirmedBooking()
	cancelled := b
	cancelled.Status = enum.BookingStatusCancelled

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)
	s.canceller.EXPECT().CancelBooking(gomock.Any(), s.bookingID).Return(cancelled, nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), s.paymentID).Return(nil)

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

func (s *cancelBookingUseCaseSuite) TestExecute_LessThanOneHourBefore_WindowExpiredError() {
	b := s.confirmedBooking()
	// Move start to 30 minutes from now — past the 1-hour cutoff.
	b.Start = time.Now().UTC().Add(30 * time.Minute)
	b.End = b.Start.Add(time.Hour)

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("CANCELLATION_WINDOW_EXPIRED", errors.InvalidInputCode(err))
}

func (s *cancelBookingUseCaseSuite) TestExecute_ExactlyAtCutoff_WindowExpiredError() {
	b := s.confirmedBooking()
	// Exactly at the 1-hour mark is past the deadline (deadline = start - 1h, and now == deadline).
	b.Start = time.Now().UTC().Add(time.Hour)
	b.End = b.Start.Add(time.Hour)

	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(b, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("CANCELLATION_WINDOW_EXPIRED", errors.InvalidInputCode(err))
}

func (s *cancelBookingUseCaseSuite) TestExecute_BookingNotFound_ReturnsNotFoundError() {
	s.canceller.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsNotFound(err))
}
