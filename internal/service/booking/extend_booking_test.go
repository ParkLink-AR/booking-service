package booking_test

import (
	"context"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

func (s *serviceSuite) TestCreateBooking_DayBlockedByOwner_Rejected() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(12 * time.Hour)
	s.blockedDates = []entity.DateRange{{From: "2026-08-17", To: "2026-08-18"}}
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		UserID:              "11111111-1111-1111-1111-111111111111",
		Start:               start,
		End:                 end,
	}
	s.parkingClient.EXPECT().IsSpotActive(gomock.Any(), s.parkingSpotID).Return(true, nil)

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("SPOT_DAY_BLOCKED", errors.InvalidInputCode(err))
}

// extensionParent is a Tuesday 10:00-12:00 booking on a spot open 08:00-20:00.
func (s *serviceSuite) extensionParent() entity.Booking {
	return entity.Booking{
		ID:            "parent-id",
		ParkingSpotID: s.parkingSpotID,
		UserID:        "11111111-1111-1111-1111-111111111111",
		Start:         s.localDate(18).Add(10 * time.Hour),
		End:           s.localDate(18).Add(12 * time.Hour),
		Status:        enum.BookingStatusConfirmed,
		Kind:          enum.BookingKindHourly,
	}
}

func (s *serviceSuite) expectExtensionRules(rules entity.SpotBookingRules, active []entity.TimeWindow) {
	parent := s.extensionParent()
	s.parkingClient.EXPECT().GetSpotBookingRules(gomock.Any(), s.parkingSpotID).Return(rules, nil)
	s.parkingClient.EXPECT().GetHourlyPrice(gomock.Any(), s.parkingSpotID, "").Return(100.0, nil)
	s.commissionRates.EXPECT().GetCommissionRate(gomock.Any()).Return(entity.PlatformSettings{CommissionRateBP: 1000}, nil)
	s.bookingRepository.EXPECT().
		GetActiveBookingsInRangeExcluding(gomock.Any(), s.parkingSpotID, parent.End, parent.End.Add(12*time.Hour), "parent-id").
		Return(active, nil)
}

func (s *serviceSuite) TestExtensionOptions_StopsAtClosingTime() {
	s.expectExtensionRules(entity.SpotBookingRules{Active: true, HourlyEnabled: true, Schedule: tuesdaySchedule, Capacity: 1}, nil)

	options, err := s.service.ExtensionOptions(s.context, s.extensionParent())

	s.Require().NoError(err)
	s.Require().Len(options, 8) // 12:00 -> 13:00 ... 20:00
	s.Equal(s.localDate(18).Add(13*time.Hour), options[0].End)
	s.Equal(11000, options[0].Amount) // $100/h plus 10%
	s.Equal(s.localDate(18).Add(20*time.Hour), options[7].End)
}

// The next booking (padded by the grace period) ends the options where it starts.
func (s *serviceSuite) TestExtensionOptions_StopsAtNextBooking() {
	next := entity.OccupancyWindow(entity.TimeWindow{
		Start: s.localDate(18).Add(15*time.Hour + 15*time.Minute),
		End:   s.localDate(18).Add(17 * time.Hour),
	})
	s.expectExtensionRules(entity.SpotBookingRules{Active: true, HourlyEnabled: true, Schedule: tuesdaySchedule, Capacity: 1},
		[]entity.TimeWindow{next})

	options, err := s.service.ExtensionOptions(s.context, s.extensionParent())

	s.Require().NoError(err)
	s.Require().Len(options, 3) // until 15:00
}

func (s *serviceSuite) TestExtensionOptions_SpotPaused_Rejected() {
	s.parkingClient.EXPECT().GetSpotBookingRules(gomock.Any(), s.parkingSpotID).
		Return(entity.SpotBookingRules{Active: false, HourlyEnabled: true}, nil)

	_, err := s.service.ExtensionOptions(s.context, s.extensionParent())

	s.True(errors.IsInvalidInput(err))
}

func (s *serviceSuite) TestCreateExtension_ChainsToParentAwaitingPayment() {
	s.expectExtensionRules(entity.SpotBookingRules{Active: true, HourlyEnabled: true, Schedule: tuesdaySchedule, Capacity: 1}, nil)
	end := s.localDate(18).Add(14 * time.Hour)
	s.bookingRepository.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, booking entity.Booking) (entity.Booking, error) {
			s.Require().NotNil(booking.ParentBookingID)
			s.Equal("parent-id", *booking.ParentBookingID)
			s.Equal(s.localDate(18).Add(12*time.Hour).UTC(), booking.Start)
			s.Equal(end.UTC(), booking.End)
			s.Equal(enum.BookingStatusPendingPayment, booking.Status)
			s.Equal(22000, booking.Amount)
			booking.ID = "extension-id"
			return booking, nil
		})

	got, err := s.service.CreateExtension(s.context, s.extensionParent(), end)

	s.Require().NoError(err)
	s.Equal("extension-id", got.ID)
}

func (s *serviceSuite) TestCreateExtension_EndNotOffered_Rejected() {
	s.expectExtensionRules(entity.SpotBookingRules{Active: true, HourlyEnabled: true, Schedule: tuesdaySchedule, Capacity: 1}, nil)

	_, err := s.service.CreateExtension(s.context, s.extensionParent(), s.localDate(18).Add(22*time.Hour))

	s.True(errors.IsInvalidInput(err))
	s.Equal("EXTENSION_NOT_AVAILABLE", errors.InvalidInputCode(err))
}
