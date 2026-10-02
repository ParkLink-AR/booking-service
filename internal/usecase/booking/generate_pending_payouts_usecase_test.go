package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestGeneratePendingPayoutsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(generatePendingPayoutsUseCaseSuite))
}

type generatePendingPayoutsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	bookings   *mocks.MockEligibleBookingLister
	payouts    *mocks.MockPayoutCreator
	settings   *mocks.MockCommissionRateGetter
	spots      *mocks.MockSpotDetailsLookup
	useCase    *booking.GeneratePendingPayoutsUseCase
	context    context.Context
}

func (s *generatePendingPayoutsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookings = mocks.NewMockEligibleBookingLister(s.controller)
	s.payouts = mocks.NewMockPayoutCreator(s.controller)
	s.settings = mocks.NewMockCommissionRateGetter(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.useCase = booking.NewGeneratePendingPayoutsUseCase(s.bookings, s.payouts, s.settings, s.spots)
	s.context = context.Background()
}

func (s *generatePendingPayoutsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *generatePendingPayoutsUseCaseSuite) TestExecute_NoEligibleBookings_CreatesNoneNoRateLookup() {
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).Return(nil, nil)

	created, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(0, created)
}

func (s *generatePendingPayoutsUseCaseSuite) TestExecute_ListError_PropagatesAndCreatesNone() {
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).
		Return(nil, errors.NewInternalError("boom", context.DeadlineExceeded))

	created, err := s.useCase.Execute(s.context)

	s.Equal(0, created)
	s.Error(err)
}

func (s *generatePendingPayoutsUseCaseSuite) TestExecute_CommissionRateError_PropagatesAndCreatesNone() {
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).
		Return([]entity.Booking{{ID: "b1", ParkingSpotID: "spot1", Amount: 10000}}, nil)
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{}, errors.NewInternalError("boom", context.DeadlineExceeded))

	created, err := s.useCase.Execute(s.context)

	s.Equal(0, created)
	s.Error(err)
}

func (s *generatePendingPayoutsUseCaseSuite) TestExecute_ComputesCommissionAtConfiguredRate() {
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).
		Return([]entity.Booking{{ID: "b1", ParkingSpotID: "spot1", Amount: 10000, PaymentID: "mp-payment-123"}}, nil)
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot1").
		Return(entity.SpotDetails{OwnerID: "owner1", Title: "Cochera"}, nil)

	var captured entity.Payout
	s.payouts.EXPECT().CreatePayout(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payout entity.Payout) (entity.Payout, error) {
			captured = payout
			return payout, nil
		})

	created, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(1, created)
	s.Equal("b1", captured.BookingID)
	s.Equal("owner1", captured.OwnerID)
	s.Equal("mp-payment-123", captured.PaymentID)
	// b.Amount (10000) is the driver's inclusive total -- the owner's price plus 5%
	// commission already added on top at booking time. Backing the commission out of
	// that total (rate/(10000+rate), not a straight rate/10000 cut of it) is what makes
	// NetAmount land on the owner's original ~9524, not short them a second 5% on top of
	// the one already collected from the driver: 10000*500/10500 = 476 (int division).
	s.Equal(10000, captured.GrossAmount)
	s.Equal(476, captured.CommissionAmount)
	s.Equal(9524, captured.NetAmount)
}

// A booking created with the owner_amount column populated must have its payout derived
// from that stored value, never from whatever commission rate happens to be configured
// when this sweep runs -- otherwise an admin raising/lowering the rate after the booking
// was made would retroactively change what its owner gets paid. The rate returned here
// (2000bp = 20%) would back out a very different commission than OwnerAmount implies,
// proving it's ignored for this booking.
func (s *generatePendingPayoutsUseCaseSuite) TestExecute_UsesStoredOwnerAmount_IgnoresCurrentRate() {
	ownerAmount := 10000
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).
		Return([]entity.Booking{{
			ID: "b1", ParkingSpotID: "spot1", Amount: 10500, OwnerAmount: &ownerAmount, PaymentID: "mp-payment-123",
		}}, nil)
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 2000}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot1").
		Return(entity.SpotDetails{OwnerID: "owner1", Title: "Cochera"}, nil)

	var captured entity.Payout
	s.payouts.EXPECT().CreatePayout(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payout entity.Payout) (entity.Payout, error) {
			captured = payout
			return payout, nil
		})

	created, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(1, created)
	s.Equal(10500, captured.GrossAmount)
	s.Equal(500, captured.CommissionAmount)
	s.Equal(10000, captured.NetAmount)
}

func (s *generatePendingPayoutsUseCaseSuite) TestExecute_OneFailingSpotLookup_DoesNotStopTheRest() {
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).
		Return([]entity.Booking{
			{ID: "b1", ParkingSpotID: "spot1", Amount: 10000},
			{ID: "b2", ParkingSpotID: "spot2", Amount: 20000},
		}, nil)
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot1").
		Return(entity.SpotDetails{}, errors.NewUpstream("parkingclient.GetSpotDetails", context.DeadlineExceeded))
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot2").
		Return(entity.SpotDetails{OwnerID: "owner2", Title: "Cochera 2"}, nil)
	s.payouts.EXPECT().CreatePayout(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payout entity.Payout) (entity.Payout, error) {
			return payout, nil
		})

	created, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(1, created)
}

func (s *generatePendingPayoutsUseCaseSuite) TestExecute_SkipsBookingThatAlreadyHasPayout() {
	// ListEligibleBookingsWithoutPayout already excludes bookings with a payout row (via its
	// LEFT JOIN), so a booking that already has one is simply never returned here -- this
	// verifies the use case creates nothing when the repository reports no eligible bookings.
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).Return([]entity.Booking{}, nil)

	created, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(0, created)
}

func (s *generatePendingPayoutsUseCaseSuite) TestExecute_FailingPayoutCreate_DoesNotStopTheRest() {
	s.bookings.EXPECT().ListEligibleBookingsWithoutPayout(gomock.Any()).
		Return([]entity.Booking{
			{ID: "b1", ParkingSpotID: "spot1", Amount: 10000},
			{ID: "b2", ParkingSpotID: "spot2", Amount: 20000},
		}, nil)
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot1").
		Return(entity.SpotDetails{OwnerID: "owner1", Title: "Cochera 1"}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot2").
		Return(entity.SpotDetails{OwnerID: "owner2", Title: "Cochera 2"}, nil)
	s.payouts.EXPECT().CreatePayout(gomock.Any(), gomock.Any()).
		Return(entity.Payout{}, errors.NewInternalError("boom", context.DeadlineExceeded))
	s.payouts.EXPECT().CreatePayout(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payout entity.Payout) (entity.Payout, error) {
			return payout, nil
		})

	created, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(1, created)
}
