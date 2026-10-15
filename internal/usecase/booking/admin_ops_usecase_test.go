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

func TestAdminOpsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(adminOpsUseCaseSuite))
}

type adminOpsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	bookings   *mocks.MockAdminBookingStore
	refunder   *mocks.MockPaymentAmountRefunder
	messages   *mocks.MockBookingMessageLister
	reviews    *mocks.MockAdminReviewStore
	reports    *mocks.MockReportStore
	metrics    *mocks.MockMetricsReader
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.AdminOpsUseCase
	context    context.Context

	bookingID string
}

func (s *adminOpsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookings = mocks.NewMockAdminBookingStore(s.controller)
	s.refunder = mocks.NewMockPaymentAmountRefunder(s.controller)
	s.messages = mocks.NewMockBookingMessageLister(s.controller)
	s.reviews = mocks.NewMockAdminReviewStore(s.controller)
	s.reports = mocks.NewMockReportStore(s.controller)
	s.metrics = mocks.NewMockMetricsReader(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewAdminOpsUseCase(s.bookings, s.refunder, s.messages, s.reviews, s.reports, s.metrics,
		s.spots, s.events, time.UTC)
	s.context = context.Background()
	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *adminOpsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

// A paid booking: $10.000 charged, $8.000 for the owner.
func (s *adminOpsUseCaseSuite) paidBooking(status enum.BookingStatus, refunded int) entity.AdminBooking {
	paidAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	ownerAmount := 800000
	return entity.AdminBooking{Booking: entity.Booking{
		ID: s.bookingID, ParkingSpotID: "spot-1", UserID: "driver-1", Status: status,
		Amount: 1000000, OwnerAmount: &ownerAmount, PaymentID: "mp-1", PaidAt: &paidAt, RefundedAmount: refunded,
	}}
}

func (s *adminOpsUseCaseSuite) expectAnnounced(action string, refunded int) {
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: "owner-1", Title: "Cochera"}, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingAdminAction, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal(action, event["action"])
			s.Equal(float64(refunded), event["refunded_amount"])
			return nil
		},
	)
}

func (s *adminOpsUseCaseSuite) TestCancelBooking_RefundsEverythingByDefault() {
	booking := s.paidBooking(enum.BookingStatusConfirmed, 0)
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(booking, nil)
	s.bookings.EXPECT().AdminCancelBooking(gomock.Any(), s.bookingID, 1000000, "Cochera inundada").Return(booking, nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "mp-1").Return(nil)
	s.expectAnnounced("cancelled", 1000000)

	_, err := s.useCase.CancelBooking(s.context, s.bookingID, nil, "  Cochera inundada ")

	s.Require().NoError(err)
}

func (s *adminOpsUseCaseSuite) TestCancelBooking_PartialRefund() {
	booking := s.paidBooking(enum.BookingStatusConfirmed, 0)
	partial := 400000
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(booking, nil)
	s.bookings.EXPECT().AdminCancelBooking(gomock.Any(), s.bookingID, partial, "Acuerdo con el conductor").Return(booking, nil)
	s.refunder.EXPECT().RefundPaymentAmount(gomock.Any(), "mp-1", partial).Return(nil)
	s.expectAnnounced("cancelled", partial)

	_, err := s.useCase.CancelBooking(s.context, s.bookingID, &partial, "Acuerdo con el conductor")

	s.Require().NoError(err)
}

func (s *adminOpsUseCaseSuite) TestCancelBooking_NeedsAReason() {
	_, err := s.useCase.CancelBooking(s.context, s.bookingID, nil, "   ")

	s.Equal("NOTE_REQUIRED", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestCancelBooking_RefundAboveWhatWasPaid_Refused() {
	tooMuch := 2000000
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(s.paidBooking(enum.BookingStatusConfirmed, 0), nil)

	_, err := s.useCase.CancelBooking(s.context, s.bookingID, &tooMuch, "x")

	s.Equal("INVALID_REFUND", errors.InvalidInputCode(err))
}

// The owner's payout exists but isn't in any settlement: it shrinks with the refund.
func (s *adminOpsUseCaseSuite) TestRefundBooking_PendingPayoutIsReduced() {
	booking := s.paidBooking(enum.BookingStatusCompleted, 0)
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(booking, nil)
	s.bookings.EXPECT().GetPayoutByBooking(gomock.Any(), s.bookingID).Return(&entity.Payout{
		ID: "payout-1", GrossAmount: 1000000, CommissionAmount: 200000, NetAmount: 800000, Status: enum.PayoutStatusPending,
	}, nil)
	s.refunder.EXPECT().RefundPaymentAmount(gomock.Any(), "mp-1", 250000).Return(nil)
	s.bookings.EXPECT().RecordAdminRefund(gomock.Any(), s.bookingID, 250000, "Llegó tarde el propietario").
		Return(s.paidBooking(enum.BookingStatusCompleted, 250000), nil)
	// $7.500 left: the owner keeps 80% of it.
	s.bookings.EXPECT().AdjustPayout(gomock.Any(), "payout-1", 750000, 150000, 600000).Return(nil)
	s.expectAnnounced("refunded", 250000)

	_, absorbed, err := s.useCase.RefundBooking(s.context, s.bookingID, 250000, "Llegó tarde el propietario")

	s.Require().NoError(err)
	s.False(absorbed)
}

func (s *adminOpsUseCaseSuite) TestRefundBooking_PayoutInSettlementDraft_Conflict() {
	settlementID := "settlement-1"
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(s.paidBooking(enum.BookingStatusCompleted, 0), nil)
	s.bookings.EXPECT().GetPayoutByBooking(gomock.Any(), s.bookingID).Return(&entity.Payout{
		ID: "payout-1", Status: enum.PayoutStatusPending, SettlementID: &settlementID,
	}, nil)

	_, _, err := s.useCase.RefundBooking(s.context, s.bookingID, 100000, "x")

	s.True(errors.IsConflict(err))
}

func (s *adminOpsUseCaseSuite) TestRefundBooking_OwnerAlreadyPaid_ParkLinkAbsorbsIt() {
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(s.paidBooking(enum.BookingStatusCompleted, 0), nil)
	s.bookings.EXPECT().GetPayoutByBooking(gomock.Any(), s.bookingID).Return(&entity.Payout{
		ID: "payout-1", Status: enum.PayoutStatusPaid,
	}, nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "mp-1").Return(nil)
	s.bookings.EXPECT().RecordAdminRefund(gomock.Any(), s.bookingID, 1000000, "x").
		Return(s.paidBooking(enum.BookingStatusCompleted, 1000000), nil)
	s.expectAnnounced("refunded", 1000000)

	_, absorbed, err := s.useCase.RefundBooking(s.context, s.bookingID, 1000000, "x")

	s.Require().NoError(err)
	s.True(absorbed)
}

func (s *adminOpsUseCaseSuite) TestRefundBooking_BookingNotFinished_UseCancelInstead() {
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(s.paidBooking(enum.BookingStatusConfirmed, 0), nil)

	_, _, err := s.useCase.RefundBooking(s.context, s.bookingID, 100000, "x")

	s.Equal("NOT_FINISHED", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestRefundBooking_MoreThanWhatsLeft_Refused() {
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), s.bookingID).Return(s.paidBooking(enum.BookingStatusCompleted, 900000), nil)

	_, _, err := s.useCase.RefundBooking(s.context, s.bookingID, 200000, "x")

	s.Equal("INVALID_REFUND", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestCreateReport_Validates() {
	_, err := s.useCase.CreateReport(s.context, "user-1", "payment", "x", "Spam", "")
	s.Equal("INVALID_TARGET", errors.InvalidInputCode(err))

	_, err = s.useCase.CreateReport(s.context, "user-1", "user", "user-1", "Spam", "")
	s.Equal("INVALID_TARGET", errors.InvalidInputCode(err), "nobody reports themselves")

	_, err = s.useCase.CreateReport(s.context, "user-1", "spot", "spot-1", "  ", "")
	s.Equal("INVALID_REASON", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestCreateReport_Stores() {
	s.reports.EXPECT().CreateReport(gomock.Any(), entity.Report{
		ReporterID: "user-1", TargetType: "review", TargetID: "review-1", Reason: "Ofensiva", Details: "Insulta",
	}).Return(entity.Report{ID: "report-1"}, nil)

	got, err := s.useCase.CreateReport(s.context, "user-1", "review", "review-1", " Ofensiva ", " Insulta ")

	s.Require().NoError(err)
	s.Equal("report-1", got.ID)
}

func (s *adminOpsUseCaseSuite) startedBooking(status enum.BookingStatus) entity.AdminBooking {
	return entity.AdminBooking{Booking: entity.Booking{
		ID: "booking-1", ParkingSpotID: "spot-1", UserID: "driver-1", Status: status,
		Start: time.Now().Add(-time.Hour), End: time.Now().Add(time.Hour),
	}}
}

// The spot's owner reports the driver never came.
func (s *adminOpsUseCaseSuite) TestCreateReport_BookingByItsOwner_Stores() {
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), "booking-1").Return(s.startedBooking(enum.BookingStatusConfirmed), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: "owner-1"}, nil)
	s.reports.EXPECT().CreateReport(gomock.Any(), entity.Report{
		ReporterID: "owner-1", TargetType: "booking", TargetID: "booking-1", Reason: "El conductor no vino",
	}).Return(entity.Report{ID: "report-1"}, nil)

	got, err := s.useCase.CreateReport(s.context, "owner-1", "booking", "booking-1", "El conductor no vino", "")

	s.Require().NoError(err)
	s.Equal("report-1", got.ID)
}

// The driver reports the spot was taken -- no owner lookup needed.
func (s *adminOpsUseCaseSuite) TestCreateReport_BookingByItsDriver_Stores() {
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), "booking-1").Return(s.startedBooking(enum.BookingStatusInProgress), nil)
	s.reports.EXPECT().CreateReport(gomock.Any(), gomock.Any()).Return(entity.Report{ID: "report-2"}, nil)

	_, err := s.useCase.CreateReport(s.context, "driver-1", "booking", "booking-1", "La cochera estaba ocupada", "")

	s.Require().NoError(err)
}

func (s *adminOpsUseCaseSuite) TestCreateReport_BookingBySomeoneElse_Forbidden() {
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), "booking-1").Return(s.startedBooking(enum.BookingStatusConfirmed), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: "owner-1"}, nil)

	_, err := s.useCase.CreateReport(s.context, "stranger", "booking", "booking-1", "Algo", "")

	s.True(errors.IsForbidden(err))
}

func (s *adminOpsUseCaseSuite) TestCreateReport_BookingNotStartedYet_Rejected() {
	booking := s.startedBooking(enum.BookingStatusConfirmed)
	booking.Start = time.Now().Add(3 * time.Hour)
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), "booking-1").Return(booking, nil)

	_, err := s.useCase.CreateReport(s.context, "driver-1", "booking", "booking-1", "Algo", "")

	s.Equal("BOOKING_NOT_STARTED", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestCreateReport_CancelledBooking_Rejected() {
	s.bookings.EXPECT().GetAdminBooking(gomock.Any(), "booking-1").Return(s.startedBooking(enum.BookingStatusCancelled), nil)

	_, err := s.useCase.CreateReport(s.context, "driver-1", "booking", "booking-1", "Algo", "")

	s.Equal("BOOKING_NOT_REPORTABLE", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestResolveReport_OnlyFinalStatuses() {
	_, err := s.useCase.ResolveReport(s.context, "report-1", "PENDING", "", "admin-1")

	s.Equal("INVALID_STATUS", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestHideReview_NeedsAReason() {
	err := s.useCase.HideReview(s.context, "review-1", "", "admin-1")

	s.Equal("NOTE_REQUIRED", errors.InvalidInputCode(err))
}

func (s *adminOpsUseCaseSuite) TestMetrics_DefaultsToThirtyDays() {
	s.metrics.EXPECT().GetAdminMetrics(gomock.Any(), 30, time.UTC).Return(entity.AdminMetrics{Days: 30}, nil)

	got, err := s.useCase.Metrics(s.context, 400)

	s.Require().NoError(err)
	s.Equal(30, got.Days)
}
