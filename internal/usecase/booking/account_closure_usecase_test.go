package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

const (
	closingUserID = "11111111-1111-1111-1111-111111111111"
	otherUserID   = "22222222-2222-2222-2222-222222222222"
	ownSpotID     = "33333333-3333-3333-3333-333333333333"
	otherSpotID   = "44444444-4444-4444-4444-444444444444"
)

func TestAccountClosureUseCaseSuite(t *testing.T) {
	suite.Run(t, new(accountClosureUseCaseSuite))
}

type accountClosureUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	repository *mocks.MockAccountClosureRepository
	refunder   *mocks.MockPaymentRefunder
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.AccountClosureUseCase
	context    context.Context
	spotIDs    []string
}

func (s *accountClosureUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.repository = mocks.NewMockAccountClosureRepository(s.controller)
	s.refunder = mocks.NewMockPaymentRefunder(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewAccountClosureUseCase(s.repository, s.refunder, s.spots, s.events)
	s.context = context.Background()
	s.spotIDs = []string{ownSpotID}
}

func (s *accountClosureUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

// upcoming returns a booking that starts in an hour, as the closing user's driver booking
// (asDriver) or on their spot by someone else.
func upcoming(id string, status enum.BookingStatus, asDriver bool) entity.Booking {
	start := time.Now().UTC().Add(time.Hour)
	b := entity.Booking{ID: id, Status: status, Start: start, End: start.Add(time.Hour), PaymentID: "pay-" + id}
	if asDriver {
		b.UserID, b.ParkingSpotID = closingUserID, otherSpotID
	} else {
		b.UserID, b.ParkingSpotID = otherUserID, ownSpotID
	}
	return b
}

func startedConfirmed(id string) entity.Booking {
	b := upcoming(id, enum.BookingStatusConfirmed, true)
	b.Start = time.Now().UTC().Add(-30 * time.Minute)
	b.End = b.Start.Add(time.Hour)
	return b
}

func endedConfirmed(id string) entity.Booking {
	b := upcoming(id, enum.BookingStatusConfirmed, false)
	b.Start = time.Now().UTC().Add(-3 * time.Hour)
	b.End = b.Start.Add(time.Hour)
	return b
}

type blockingBookingCase struct {
	name    string
	booking entity.Booking
}

var blockingBookingCases = []blockingBookingCase{
	{name: "in progress", booking: upcoming("b1", enum.BookingStatusInProgress, false)},
	{name: "confirmed and already started (car parked, code not entered)", booking: startedConfirmed("b2")},
}

func (s *accountClosureUseCaseSuite) TestSummary_CountsWhatWouldHappen() {
	s.repository.EXPECT().ListAccountBookings(gomock.Any(), closingUserID, s.spotIDs).Return([]entity.Booking{
		upcoming("driver-confirmed", enum.BookingStatusConfirmed, true),
		upcoming("driver-pending", enum.BookingStatusPendingOwnerApproval, true),
		upcoming("owner-confirmed", enum.BookingStatusConfirmed, false),
		upcoming("owner-request", enum.BookingStatusPendingOwnerApproval, false),
		upcoming("owner-in-progress", enum.BookingStatusInProgress, false),
		upcoming("driver-hold", enum.BookingStatusPendingPayment, true),
		endedConfirmed("owner-ended"),
	}, nil)
	s.repository.EXPECT().GetOwnerPendingPayouts(gomock.Any(), closingUserID, s.spotIDs).Return(2, 150000, nil)

	summary, err := s.useCase.Summary(s.context, closingUserID, s.spotIDs)

	s.Require().NoError(err)
	s.Equal(entity.AccountClosureSummary{
		InProgressCount:        1,
		DriverBookingsToCancel: 2,
		OwnerBookingsToCancel:  1,
		OwnerRequestsToReject:  1,
		PendingPayoutCount:     2,
		PendingPayoutAmount:    150000,
	}, summary)
}

func (s *accountClosureUseCaseSuite) TestCancelBookings_RefusedWhileABookingIsInProgress() {
	for _, testCase := range blockingBookingCases {
		s.repository.EXPECT().ListAccountBookings(gomock.Any(), closingUserID, s.spotIDs).
			Return([]entity.Booking{upcoming("other", enum.BookingStatusConfirmed, true), testCase.booking}, nil)

		_, err := s.useCase.CancelBookings(s.context, closingUserID, s.spotIDs)

		s.True(errors.IsConflict(err), testCase.name)
	}
}

func (s *accountClosureUseCaseSuite) TestCancelBookings_CancelsAndRefundsEverythingNotStarted() {
	driverBooking := upcoming("driver-confirmed", enum.BookingStatusConfirmed, true)
	ownerBooking := upcoming("owner-confirmed", enum.BookingStatusConfirmed, false)
	ownerRequest := upcoming("owner-request", enum.BookingStatusPendingOwnerApproval, false)
	hold := upcoming("driver-hold", enum.BookingStatusPendingPayment, true)
	s.repository.EXPECT().ListAccountBookings(gomock.Any(), closingUserID, s.spotIDs).
		Return([]entity.Booking{driverBooking, ownerBooking, ownerRequest, hold, endedConfirmed("ended")}, nil)

	s.repository.EXPECT().CancelBookingWithReason(gomock.Any(), driverBooking.ID, constants.CancellationReasonDriverAccountClosed).Return(driverBooking, nil)
	s.repository.EXPECT().CancelBookingWithReason(gomock.Any(), ownerBooking.ID, constants.CancellationReasonOwnerAccountClosed).Return(ownerBooking, nil)
	s.repository.EXPECT().CancelBookingWithReason(gomock.Any(), ownerRequest.ID, constants.CancellationReasonOwnerAccountClosed).Return(ownerRequest, nil)
	s.repository.EXPECT().ExpireHold(gomock.Any(), hold.ID).Return(nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), driverBooking.PaymentID).Return(nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), ownerBooking.PaymentID).Return(nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), ownerRequest.PaymentID).Return(nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), gomock.Any()).Return(entity.SpotDetails{Title: "Cochera"}, nil).Times(3)

	cancelledBy := map[string]string{}
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCancelled, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, payload any) error {
			raw, _ := json.Marshal(payload)
			var event struct {
				BookingID   string `json:"booking_id"`
				CancelledBy string `json:"cancelled_by"`
			}
			_ = json.Unmarshal(raw, &event)
			cancelledBy[event.BookingID] = event.CancelledBy
			return nil
		}).Times(3)

	result, err := s.useCase.CancelBookings(s.context, closingUserID, s.spotIDs)

	s.Require().NoError(err)
	s.Equal(entity.AccountClosureResult{Cancelled: 3, ExpiredHolds: 1}, result)
	s.Equal(map[string]string{
		driverBooking.ID: "driver_account",
		ownerBooking.ID:  "owner_account",
		ownerRequest.ID:  "owner_account",
	}, cancelledBy)
}

func (s *accountClosureUseCaseSuite) TestCancelBookings_AFailedRefundDoesNotStopTheRest() {
	first := upcoming("first", enum.BookingStatusConfirmed, false)
	second := upcoming("second", enum.BookingStatusConfirmed, false)
	s.repository.EXPECT().ListAccountBookings(gomock.Any(), closingUserID, s.spotIDs).Return([]entity.Booking{first, second}, nil)
	s.repository.EXPECT().CancelBookingWithReason(gomock.Any(), first.ID, gomock.Any()).Return(first, nil)
	s.repository.EXPECT().CancelBookingWithReason(gomock.Any(), second.ID, gomock.Any()).Return(second, nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), first.PaymentID).Return(errors.NewUpstream("payment", context.DeadlineExceeded))
	s.refunder.EXPECT().RefundPayment(gomock.Any(), second.PaymentID).Return(nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), gomock.Any()).Return(entity.SpotDetails{}, nil).Times(2)
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

	result, err := s.useCase.CancelBookings(s.context, closingUserID, s.spotIDs)

	s.Require().NoError(err)
	s.Equal(2, result.Cancelled)
	s.Equal([]string{first.ID}, result.RefundFailures)
}

func (s *accountClosureUseCaseSuite) TestCancelBookings_SkipsABookingThatChangedMeanwhile() {
	gone := upcoming("gone", enum.BookingStatusConfirmed, true)
	s.repository.EXPECT().ListAccountBookings(gomock.Any(), closingUserID, s.spotIDs).Return([]entity.Booking{gone}, nil)
	s.repository.EXPECT().CancelBookingWithReason(gomock.Any(), gone.ID, gomock.Any()).Return(entity.Booking{}, errors.NewNotFound("booking", gone.ID))

	result, err := s.useCase.CancelBookings(s.context, closingUserID, s.spotIDs)

	s.Require().NoError(err)
	s.Equal(0, result.Cancelled)
}

func (s *accountClosureUseCaseSuite) TestAnonymize_ClearsTheVehicleSnapshot() {
	s.repository.EXPECT().ClearVehicleSnapshot(gomock.Any(), closingUserID).Return(nil)

	s.NoError(s.useCase.Anonymize(s.context, closingUserID))
}
