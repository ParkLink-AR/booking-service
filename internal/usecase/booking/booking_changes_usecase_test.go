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

func TestBookingChangesUseCaseSuite(t *testing.T) {
	suite.Run(t, new(bookingChangesUseCaseSuite))
}

type bookingChangesUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	bookings   *mocks.MockBookingChangesRepository
	extender   *mocks.MockBookingExtender
	spots      *mocks.MockBookingChangesSpots
	refunder   *mocks.MockPaymentRefunder
	events     *mocks.MockEventPublisher
	useCase    *booking.BookingChangesUseCase
	context    context.Context
}

const (
	changesBookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	changesDriverID  = "22222222-2222-2222-2222-222222222222"
	changesOwnerID   = "33333333-3333-3333-3333-333333333333"
	changesSpotID    = "spot-1"
)

func (s *bookingChangesUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookings = mocks.NewMockBookingChangesRepository(s.controller)
	s.extender = mocks.NewMockBookingExtender(s.controller)
	s.spots = mocks.NewMockBookingChangesSpots(s.controller)
	s.refunder = mocks.NewMockPaymentRefunder(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewBookingChangesUseCase(s.bookings, s.extender, s.spots, s.refunder, s.events)
	s.context = context.Background()
}

func (s *bookingChangesUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *bookingChangesUseCaseSuite) confirmed(startIn time.Duration) entity.Booking {
	start := time.Now().Add(startIn)
	return entity.Booking{
		ID: changesBookingID, ParkingSpotID: changesSpotID, UserID: changesDriverID, PaymentID: "pay-1",
		Start: start, End: start.Add(2 * time.Hour), Status: enum.BookingStatusConfirmed, Kind: enum.BookingKindHourly,
	}
}

func (s *bookingChangesUseCaseSuite) expectSpot() {
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), changesSpotID).Return(entity.SpotDetails{OwnerID: changesOwnerID, Title: "Cochera"}, nil)
}

func (s *bookingChangesUseCaseSuite) TestOwnerCancel_RefundsDriverInFullAndTellsThem() {
	b := s.confirmed(3 * time.Hour)
	note := "Se me rompió el portón"
	cancelled := b
	cancelled.Status = enum.BookingStatusCancelled
	cancelled.CancellationNote = &note

	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(b, nil)
	s.expectSpot()
	s.bookings.EXPECT().OwnerCancelBooking(gomock.Any(), changesBookingID, note).Return(cancelled, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCancelled, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal(booking.CancelledByOwner, event["cancelled_by"])
			s.Equal(note, event["note"])
			return nil
		})
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "pay-1").Return(nil)
	s.bookings.EXPECT().CancelExtensions(gomock.Any(), changesBookingID, constants.CancellationReasonOwnerCancelled).Return(nil, nil)

	got, err := s.useCase.OwnerCancel(s.context, changesBookingID, changesOwnerID, "  "+note+" ")

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusCancelled, got.Status)
}

func (s *bookingChangesUseCaseSuite) TestOwnerCancel_WithoutReason_Rejected() {
	_, err := s.useCase.OwnerCancel(s.context, changesBookingID, changesOwnerID, "   ")

	s.True(errors.IsInvalidInput(err))
	s.Equal("REASON_REQUIRED", errors.InvalidInputCode(err))
}

func (s *bookingChangesUseCaseSuite) TestOwnerCancel_NotTheOwner_Forbidden() {
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(s.confirmed(3*time.Hour), nil)
	s.expectSpot()

	_, err := s.useCase.OwnerCancel(s.context, changesBookingID, changesDriverID, "motivo")

	s.True(errors.IsForbidden(err))
}

func (s *bookingChangesUseCaseSuite) TestOwnerCancel_AlreadyStarted_Rejected() {
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(s.confirmed(-10*time.Minute), nil)
	s.expectSpot()

	_, err := s.useCase.OwnerCancel(s.context, changesBookingID, changesOwnerID, "motivo")

	s.Equal("ALREADY_STARTED", errors.InvalidInputCode(err))
}

func (s *bookingChangesUseCaseSuite) TestOwnerCancel_NotYetAccepted_Rejected() {
	b := s.confirmed(3 * time.Hour)
	b.Status = enum.BookingStatusPendingOwnerApproval
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(b, nil)
	s.expectSpot()

	_, err := s.useCase.OwnerCancel(s.context, changesBookingID, changesOwnerID, "motivo")

	s.Equal("NOT_CANCELLABLE", errors.InvalidInputCode(err))
}

func (s *bookingChangesUseCaseSuite) TestExtend_CreatesExtension() {
	b := s.confirmed(-30 * time.Minute)
	b.Status = enum.BookingStatusInProgress
	end := b.End.Add(time.Hour)
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(b, nil)
	s.extender.EXPECT().CreateExtension(gomock.Any(), b, end).Return(entity.Booking{ID: "ext-1"}, nil)

	got, err := s.useCase.Extend(s.context, changesBookingID, changesDriverID, end)

	s.Require().NoError(err)
	s.Equal("ext-1", got.ID)
}

func (s *bookingChangesUseCaseSuite) TestExtend_AlreadyHasExtensionAwaitingPayment_Rejected() {
	b := s.confirmed(time.Hour)
	b.Extension = &entity.BookingExtension{ID: "ext-1", Status: enum.BookingStatusPendingPayment}
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(b, nil)

	_, err := s.useCase.Extend(s.context, changesBookingID, changesDriverID, b.End.Add(time.Hour))

	s.Equal("EXTENSION_AWAITING_PAYMENT", errors.InvalidInputCode(err))
}

func (s *bookingChangesUseCaseSuite) TestExtensionOptions_StayBooking_Rejected() {
	b := s.confirmed(time.Hour)
	b.Kind = enum.BookingKindStay
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(b, nil)

	_, err := s.useCase.ExtensionOptions(s.context, changesBookingID, changesDriverID)

	s.Equal("NOT_EXTENDABLE", errors.InvalidInputCode(err))
}

func (s *bookingChangesUseCaseSuite) TestExtend_SomeoneElse_Forbidden() {
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(s.confirmed(time.Hour), nil)

	_, err := s.useCase.Extend(s.context, changesBookingID, changesOwnerID, time.Now())

	s.True(errors.IsForbidden(err))
}

func (s *bookingChangesUseCaseSuite) TestAccessInstructions_ConfirmedDriverSeesThem() {
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(s.confirmed(time.Hour), nil)
	s.spots.EXPECT().GetAccessInstructions(gomock.Any(), changesSpotID).Return("Portón 1234", nil)

	got, err := s.useCase.AccessInstructions(s.context, changesBookingID, changesDriverID)

	s.Require().NoError(err)
	s.Equal("Portón 1234", got)
}

func (s *bookingChangesUseCaseSuite) TestAccessInstructions_DriverBeforeOwnerAccepts_Forbidden() {
	b := s.confirmed(time.Hour)
	b.Status = enum.BookingStatusPendingOwnerApproval
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(b, nil)

	_, err := s.useCase.AccessInstructions(s.context, changesBookingID, changesDriverID)

	s.True(errors.IsForbidden(err))
}

func (s *bookingChangesUseCaseSuite) TestAccessInstructions_Stranger_Forbidden() {
	s.bookings.EXPECT().GetByID(gomock.Any(), changesBookingID).Return(s.confirmed(time.Hour), nil)
	s.expectSpot()

	_, err := s.useCase.AccessInstructions(s.context, changesBookingID, "someone-else")

	s.True(errors.IsForbidden(err))
}

func (s *bookingChangesUseCaseSuite) TestOwnerCancellationStats_CountsAcrossTheirSpots() {
	s.spots.EXPECT().GetSpotIDsForOwner(gomock.Any(), changesOwnerID).Return([]string{"a", "b"}, nil)
	s.bookings.EXPECT().CountOwnerCancellations(gomock.Any(), []string{"a", "b"}).
		Return(entity.OwnerCancellationStats{Total: 3, Last90Days: 1}, nil)

	got, err := s.useCase.OwnerCancellationStats(s.context, changesOwnerID)

	s.Require().NoError(err)
	s.Equal(3, got.Total)
}
