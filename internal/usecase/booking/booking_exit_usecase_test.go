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

func TestBookingExitUseCaseSuite(t *testing.T) {
	suite.Run(t, new(bookingExitUseCaseSuite))
}

type bookingExitUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	bookings   *mocks.MockBookingExitRepository
	spots      *mocks.MockBookingExitSpots
	events     *mocks.MockEventPublisher
	useCase    *booking.BookingExitUseCase
	context    context.Context
}

const (
	exitBookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	exitDriverID  = "22222222-2222-2222-2222-222222222222"
	exitOwnerID   = "33333333-3333-3333-3333-333333333333"
	exitSpotID    = "spot-1"
)

func (s *bookingExitUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookings = mocks.NewMockBookingExitRepository(s.controller)
	s.spots = mocks.NewMockBookingExitSpots(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewBookingExitUseCase(s.bookings, s.spots, s.events)
	s.context = context.Background()
}

func (s *bookingExitUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

// verified is a two-hour booking, $2.400 for the driver ($2.000 the owner's), whose arrival the
// owner verified and which ended endedAgo ago.
func (s *bookingExitUseCaseSuite) verified(endedAgo time.Duration) entity.Booking {
	end := time.Now().Add(-endedAgo)
	verifiedAt := end.Add(-2 * time.Hour)
	ownerAmount := 200000
	return entity.Booking{
		ID: exitBookingID, ParkingSpotID: exitSpotID, UserID: exitDriverID,
		Start: end.Add(-2 * time.Hour), End: end, Amount: 240000, OwnerAmount: &ownerAmount,
		Status: enum.BookingStatusInProgress, Kind: enum.BookingKindHourly, VerifiedAt: &verifiedAt,
	}
}

func (s *bookingExitUseCaseSuite) expectSpot() {
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), exitSpotID).Return(entity.SpotDetails{OwnerID: exitOwnerID, Title: "Cochera"}, nil)
}

func (s *bookingExitUseCaseSuite) TestView_DriverOfAwaitingBooking_GetsANewCode() {
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(s.verified(10*time.Minute), nil)
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(nil, nil)
	s.bookings.EXPECT().EnsureExit(gomock.Any(), exitBookingID, gomock.Any()).DoAndReturn(
		func(_ context.Context, bookingID, code string) (entity.BookingExit, error) {
			s.Regexp(`^\d{4}$`, code)
			return entity.BookingExit{BookingID: bookingID, Code: code}, nil
		})

	view, err := s.useCase.View(s.context, exitBookingID, entity.BookingViewer{UserID: exitDriverID})

	s.Require().NoError(err)
	s.Equal(entity.ExitRoleDriver, view.Role)
	s.True(view.Awaiting)
	s.Require().NotNil(view.Exit)
	s.Regexp(`^\d{4}$`, view.Exit.Code)
}

func (s *bookingExitUseCaseSuite) TestView_DriverWithAnExistingCode_SeesTheSameOne() {
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(s.verified(10*time.Minute), nil)
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(&entity.BookingExit{BookingID: exitBookingID, Code: "4821"}, nil)

	view, err := s.useCase.View(s.context, exitBookingID, entity.BookingViewer{UserID: exitDriverID})

	s.Require().NoError(err)
	s.True(view.Awaiting)
	s.Equal("4821", view.Exit.Code)
}

func (s *bookingExitUseCaseSuite) TestView_OwnerDoesNotCreateTheCode() {
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(s.verified(10*time.Minute), nil)
	s.expectSpot()
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(nil, nil)

	view, err := s.useCase.View(s.context, exitBookingID, entity.BookingViewer{UserID: exitOwnerID})

	s.Require().NoError(err)
	s.Equal(entity.ExitRoleOwner, view.Role)
	s.True(view.Awaiting)
	s.Nil(view.Exit)
}

func (s *bookingExitUseCaseSuite) TestView_SomeoneElse_Forbidden() {
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(s.verified(10*time.Minute), nil)
	s.expectSpot()

	_, err := s.useCase.View(s.context, exitBookingID, entity.BookingViewer{UserID: "44444444-4444-4444-4444-444444444444"})

	s.True(errors.IsForbidden(err))
}

func (s *bookingExitUseCaseSuite) TestView_AdminSeesItWithoutCreatingAnything() {
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(s.verified(10*time.Minute), nil)
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(nil, nil)

	view, err := s.useCase.View(s.context, exitBookingID, entity.BookingViewer{Privileged: true})

	s.Require().NoError(err)
	s.Equal(entity.ExitRoleAdmin, view.Role)
	s.True(view.Awaiting)
}

func (s *bookingExitUseCaseSuite) TestView_NeverVerifiedBooking_IsNotAwaitingAnything() {
	unverified := s.verified(10 * time.Minute)
	unverified.VerifiedAt = nil
	unverified.Status = enum.BookingStatusConfirmed
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(unverified, nil)
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(nil, nil)

	view, err := s.useCase.View(s.context, exitBookingID, entity.BookingViewer{UserID: exitDriverID})

	s.Require().NoError(err)
	s.False(view.Awaiting)
	s.Nil(view.Exit)
}

func (s *bookingExitUseCaseSuite) TestView_ClosedBookingShowsWhatItCost() {
	exitedAt := time.Now()
	closed := &entity.BookingExit{
		BookingID: exitBookingID, Code: "4821", ExitedAt: &exitedAt, ClosedBy: entity.ExitClosedByOwnerCode,
		Overstay: entity.Overstay{Minutes: 30, ChargedMinutes: 30, Amount: 60000, OwnerAmount: 50000},
	}
	completed := s.verified(30 * time.Minute)
	completed.Status = enum.BookingStatusCompleted
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(completed, nil)
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(closed, nil)

	view, err := s.useCase.View(s.context, exitBookingID, entity.BookingViewer{UserID: exitDriverID})

	s.Require().NoError(err)
	s.False(view.Awaiting)
	s.Equal(60000, view.Exit.Overstay.Amount)
}

func (s *bookingExitUseCaseSuite) TestVerify_WithinTheGracePeriod_ClosesWithoutCharging() {
	b := s.verified(5 * time.Minute)
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(b, nil)
	s.expectSpot()
	s.bookings.EXPECT().VerifyExitCode(gomock.Any(), exitBookingID, "4821").Return(nil)
	completed := b
	completed.Status = enum.BookingStatusCompleted
	s.bookings.EXPECT().CloseExit(gomock.Any(), exitBookingID, gomock.Any(), entity.ExitClosedByOwnerCode, gomock.Any(), exitOwnerID).DoAndReturn(
		func(_ context.Context, _ string, exitedAt time.Time, _ string, overstay entity.Overstay, _ string) (entity.Booking, error) {
			s.WithinDuration(time.Now(), exitedAt, 5*time.Second)
			s.Equal(0, overstay.ChargedMinutes)
			s.Zero(overstay.Amount)
			s.InDelta(5, overstay.Minutes, 1)
			return completed, nil
		})
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(&entity.BookingExit{BookingID: exitBookingID}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCompleted, gomock.Any()).Return(nil)

	got, exit, err := s.useCase.Verify(s.context, exitBookingID, exitOwnerID, "4821")

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusCompleted, got.Status)
	s.Equal(exitBookingID, exit.BookingID)
}

func (s *bookingExitUseCaseSuite) TestVerify_PastTheGracePeriod_ChargesAllTheExtraTime() {
	b := s.verified(30 * time.Minute)
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(b, nil)
	s.expectSpot()
	s.bookings.EXPECT().VerifyExitCode(gomock.Any(), exitBookingID, "4821").Return(nil)
	s.bookings.EXPECT().CloseExit(gomock.Any(), exitBookingID, gomock.Any(), entity.ExitClosedByOwnerCode, gomock.Any(), exitOwnerID).DoAndReturn(
		func(_ context.Context, _ string, _ time.Time, _ string, overstay entity.Overstay, _ string) (entity.Booking, error) {
			// Thirty-odd minutes at $2.000 an hour for the owner and $2.400 for the driver.
			s.InDelta(30, overstay.ChargedMinutes, 1)
			s.InDelta(60000, overstay.Amount, 2000)
			s.InDelta(50000, overstay.OwnerAmount, 1700)
			return b, nil
		})
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(nil, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCompleted, gomock.Any()).Return(nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingOverstayCharged, gomock.Any()).Return(nil)

	_, exit, err := s.useCase.Verify(s.context, exitBookingID, exitOwnerID, "4821")

	s.Require().NoError(err)
	s.Equal(entity.ExitClosedByOwnerCode, exit.ClosedBy)
	s.InDelta(30, exit.Overstay.ChargedMinutes, 1)
}

func (s *bookingExitUseCaseSuite) TestVerify_NotTheOwner_Forbidden() {
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(s.verified(5*time.Minute), nil)
	s.expectSpot()

	_, _, err := s.useCase.Verify(s.context, exitBookingID, exitDriverID, "4821")

	s.True(errors.IsForbidden(err))
}

func (s *bookingExitUseCaseSuite) TestVerify_WrongCode_ClosesNothing() {
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(s.verified(5*time.Minute), nil)
	s.expectSpot()
	s.bookings.EXPECT().VerifyExitCode(gomock.Any(), exitBookingID, "0000").Return(errors.NewInvalidInput("INVALID_CODE", "código incorrecto"))

	_, _, err := s.useCase.Verify(s.context, exitBookingID, exitOwnerID, "0000")

	s.True(errors.IsInvalidInput(err))
	s.Equal("INVALID_CODE", errors.InvalidInputCode(err))
}

func (s *bookingExitUseCaseSuite) TestVerify_BookingNotAwaitingItsExit_Rejected() {
	neverVerified := s.verified(5 * time.Minute)
	neverVerified.VerifiedAt = nil
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(neverVerified, nil)

	_, _, err := s.useCase.Verify(s.context, exitBookingID, exitOwnerID, "4821")

	s.Equal("NOT_AWAITING_EXIT", errors.InvalidInputCode(err))
}

func (s *bookingExitUseCaseSuite) TestVerify_ExtendedBooking_ExitBelongsToTheExtension() {
	extended := s.verified(0)
	extended.Extension = &entity.BookingExtension{ID: "ext-1", End: time.Now().Add(time.Hour), Status: enum.BookingStatusConfirmed}
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(extended, nil)

	_, _, err := s.useCase.Verify(s.context, exitBookingID, exitOwnerID, "4821")

	s.Equal("ALREADY_EXTENDED", errors.InvalidInputCode(err))
}

func (s *bookingExitUseCaseSuite) TestSupportClose_RecordsTheTimeSupportGives() {
	b := s.verified(2 * time.Hour)
	exitedAt := b.End.Add(40 * time.Minute)
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(b, nil)
	s.expectSpot()
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(nil, nil)
	s.bookings.EXPECT().EnsureExit(gomock.Any(), exitBookingID, gomock.Any()).Return(entity.BookingExit{BookingID: exitBookingID}, nil)
	s.bookings.EXPECT().CloseExit(gomock.Any(), exitBookingID, exitedAt, entity.ExitClosedBySupport,
		entity.Overstay{Minutes: 40, ChargedMinutes: 40, Amount: 80000, OwnerAmount: 66667}, exitOwnerID).Return(b, nil)
	s.bookings.EXPECT().FindExit(gomock.Any(), exitBookingID).Return(nil, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCompleted, gomock.Any()).Return(nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingOverstayCharged, gomock.Any()).Return(nil)

	_, exit, err := s.useCase.SupportClose(s.context, exitBookingID, exitedAt)

	s.Require().NoError(err)
	s.Equal(entity.ExitClosedBySupport, exit.ClosedBy)
	s.Equal(80000, exit.Overstay.Amount)
}

func (s *bookingExitUseCaseSuite) TestSupportClose_TimeBeforeTheBookingStarted_Rejected() {
	b := s.verified(2 * time.Hour)
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(b, nil)
	s.expectSpot()

	_, _, err := s.useCase.SupportClose(s.context, exitBookingID, b.Start.Add(-time.Minute))

	s.Equal("INVALID_EXIT_TIME", errors.InvalidInputCode(err))
}

func (s *bookingExitUseCaseSuite) TestSupportClose_TimeInTheFuture_Rejected() {
	b := s.verified(2 * time.Hour)
	s.bookings.EXPECT().GetByID(gomock.Any(), exitBookingID).Return(b, nil)
	s.expectSpot()

	_, _, err := s.useCase.SupportClose(s.context, exitBookingID, time.Now().Add(time.Hour))

	s.Equal("INVALID_EXIT_TIME", errors.InvalidInputCode(err))
}
