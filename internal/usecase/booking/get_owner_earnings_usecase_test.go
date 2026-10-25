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

func TestGetOwnerEarningsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getOwnerEarningsUseCaseSuite))
}

type getOwnerEarningsUseCaseSuite struct {
	suite.Suite
	controller  *gomock.Controller
	spots       *mocks.MockOwnerSpotIDsLookup
	bookings    *mocks.MockOwnerSpotBookingLister
	settlements *mocks.MockOwnerSettlementLister
	settings    *mocks.MockCommissionRateGetter
	useCase     *booking.GetOwnerEarningsUseCase
	context     context.Context
	location    *time.Location
	ownerID     string
	spotIDs     []string
}

func (s *getOwnerEarningsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.spots = mocks.NewMockOwnerSpotIDsLookup(s.controller)
	s.bookings = mocks.NewMockOwnerSpotBookingLister(s.controller)
	s.settlements = mocks.NewMockOwnerSettlementLister(s.controller)
	s.settings = mocks.NewMockCommissionRateGetter(s.controller)
	s.location = time.FixedZone("ART", -3*60*60)
	now := func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, s.location) }
	s.useCase = booking.NewGetOwnerEarningsUseCase(s.spots, s.bookings, s.settlements, s.settings, s.location, now)
	s.context = context.Background()
	s.ownerID = "33333333-3333-3333-3333-333333333333"
	s.spotIDs = []string{"spot-a", "spot-b"}
}

func (s *getOwnerEarningsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

// paidBooking is a paid booking on spotID that ended on the given day of 2026 at noon:
// the driver paid 1100 of which the owner's own price is 1000.
func (s *getOwnerEarningsUseCaseSuite) paidBooking(spotID string, month time.Month, day int, status enum.BookingStatus) entity.Booking {
	paidAt := time.Date(2026, 1, 1, 0, 0, 0, 0, s.location)
	ownerAmount := 1000
	end := time.Date(2026, month, day, 12, 0, 0, 0, s.location)
	return entity.Booking{
		ParkingSpotID: spotID, Status: status, Amount: 1100, OwnerAmount: &ownerAmount, PaidAt: &paidAt,
		Start: end.Add(-2 * time.Hour), End: end,
	}
}

func (s *getOwnerEarningsUseCaseSuite) expectData(bookings []entity.Booking, settlements []entity.Settlement) {
	s.spots.EXPECT().GetSpotIDsForOwner(gomock.Any(), s.ownerID).Return(s.spotIDs, nil)
	s.bookings.EXPECT().ListBookingsByOwnerSpots(gomock.Any(), s.spotIDs).Return(bookings, nil)
	paid := enum.SettlementStatusPaid
	s.settlements.EXPECT().ListSettlementsByOwner(gomock.Any(), s.ownerID, &paid).Return(settlements, nil)
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).Return(entity.PlatformSettings{CommissionRateBP: 1000}, nil)
}

func (s *getOwnerEarningsUseCaseSuite) TestExecute_SumsTheOwnersOwnShareByMonthAndSpot() {
	s.expectData([]entity.Booking{
		s.paidBooking("spot-a", time.October, 5, enum.BookingStatusCompleted),
		s.paidBooking("spot-a", time.October, 12, enum.BookingStatusCompleted),
		s.paidBooking("spot-b", time.August, 3, enum.BookingStatusCompleted),
	}, nil)

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Require().NoError(err)
	s.Require().Len(got.Months, 6)
	s.Equal("2026-05", got.Months[0].Month)
	s.Equal(entity.MonthEarnings{Month: "2026-10", Earned: 2000, Bookings: 2}, got.Months[5])
	s.Equal(entity.MonthEarnings{Month: "2026-08", Earned: 1000, Bookings: 1}, got.Months[3])
	s.Equal([]entity.SpotEarnings{
		{ParkingSpotID: "spot-a", Earned: 2000, Bookings: 2},
		{ParkingSpotID: "spot-b", Earned: 1000, Bookings: 1},
	}, got.Spots)
	s.Equal(3000, got.TotalEarned)
	s.Equal(3000, got.ToCollect)
}

type ownerEarningsCase struct {
	name            string
	booking         func(s *getOwnerEarningsUseCaseSuite) entity.Booking
	wantEarned      int
	wantUpcoming    int
	wantUpcomingCnt int
}

var ownerEarningsCases = []ownerEarningsCase{
	{
		name: "a late cancellation leaves the owner what wasn't refunded",
		booking: func(s *getOwnerEarningsUseCaseSuite) entity.Booking {
			b := s.paidBooking("spot-a", time.October, 5, enum.BookingStatusCancelled)
			reason := constants.CancellationReasonDriverLateCancelled
			b.CancellationReason = &reason
			b.RefundedAmount = 550
			return b
		},
		wantEarned: 500,
	},
	{
		name: "any other cancellation earns nothing",
		booking: func(s *getOwnerEarningsUseCaseSuite) entity.Booking {
			b := s.paidBooking("spot-a", time.October, 5, enum.BookingStatusCancelled)
			b.RefundedAmount = 1100
			return b
		},
	},
	{
		name: "a booking ParkLink refunded for an unconfirmed arrival earns nothing",
		booking: func(s *getOwnerEarningsUseCaseSuite) entity.Booking {
			b := s.paidBooking("spot-a", time.October, 5, enum.BookingStatusCompleted)
			review := enum.VerificationReviewRefunded
			b.VerificationReview = &review
			return b
		},
	},
	{
		name: "an accepted booking still to happen is upcoming, not earned",
		booking: func(s *getOwnerEarningsUseCaseSuite) entity.Booking {
			return s.paidBooking("spot-a", time.October, 25, enum.BookingStatusConfirmed)
		},
		wantUpcoming:    1000,
		wantUpcomingCnt: 1,
	},
	{
		name: "without the owner's price stored, the commission is backed out at today's rate",
		booking: func(s *getOwnerEarningsUseCaseSuite) entity.Booking {
			b := s.paidBooking("spot-a", time.October, 5, enum.BookingStatusCompleted)
			b.OwnerAmount = nil
			return b
		},
		wantEarned: 1000,
	},
	{
		name: "an unpaid booking counts for nothing",
		booking: func(s *getOwnerEarningsUseCaseSuite) entity.Booking {
			b := s.paidBooking("spot-a", time.October, 5, enum.BookingStatusCompleted)
			b.PaidAt = nil
			return b
		},
	},
}

func (s *getOwnerEarningsUseCaseSuite) TestExecute_WhatEachBookingCountsFor() {
	for _, tc := range ownerEarningsCases {
		s.SetupTest()
		s.expectData([]entity.Booking{tc.booking(s)}, nil)

		got, err := s.useCase.Execute(s.context, s.ownerID)

		s.Require().NoError(err, tc.name)
		s.Equal(tc.wantEarned, got.TotalEarned, tc.name)
		s.Equal(tc.wantUpcoming, got.Upcoming, tc.name)
		s.Equal(tc.wantUpcomingCnt, got.UpcomingCount, tc.name)
		s.controller.Finish()
	}
}

func (s *getOwnerEarningsUseCaseSuite) TestExecute_PaidSettlementsAreWhatsAlreadyTransferred() {
	s.expectData(
		[]entity.Booking{
			s.paidBooking("spot-a", time.October, 5, enum.BookingStatusCompleted),
			s.paidBooking("spot-a", time.September, 5, enum.BookingStatusCompleted),
		},
		[]entity.Settlement{{NetAmount: 1000, Status: enum.SettlementStatusPaid}},
	)

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Require().NoError(err)
	s.Equal(1000, got.PaidOut)
	s.Equal(1000, got.ToCollect)
}

func (s *getOwnerEarningsUseCaseSuite) TestExecute_EarningsOlderThanTheMonthsShownStillCountInTheTotal() {
	s.expectData([]entity.Booking{s.paidBooking("spot-a", time.January, 10, enum.BookingStatusCompleted)}, nil)

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Require().NoError(err)
	s.Equal(1000, got.TotalEarned)
	s.Empty(got.Spots)
	for _, month := range got.Months {
		s.Zero(month.Earned, month.Month)
	}
}

func (s *getOwnerEarningsUseCaseSuite) TestExecute_OwnerWithoutSpots_IsAllZeroWithoutQueryingBookings() {
	s.spots.EXPECT().GetSpotIDsForOwner(gomock.Any(), s.ownerID).Return([]string{}, nil)
	paid := enum.SettlementStatusPaid
	s.settlements.EXPECT().ListSettlementsByOwner(gomock.Any(), s.ownerID, &paid).Return(nil, nil)
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).Return(entity.PlatformSettings{CommissionRateBP: 1000}, nil)

	got, err := s.useCase.Execute(s.context, s.ownerID)

	s.Require().NoError(err)
	s.Len(got.Months, 6)
	s.Zero(got.TotalEarned)
	s.Empty(got.Spots)
}

func (s *getOwnerEarningsUseCaseSuite) TestExecute_SpotLookupFails_Propagates() {
	s.spots.EXPECT().GetSpotIDsForOwner(gomock.Any(), s.ownerID).
		Return(nil, errors.NewUpstream("parkingclient.GetSpotIDsForOwner", context.DeadlineExceeded))

	_, err := s.useCase.Execute(s.context, s.ownerID)

	s.Error(err)
}
