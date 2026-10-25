package booking_test

import (
	"time"

	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// The rules about when a booking can be made, all checked against the spot's own rules from
// parking-service and the service's clock (2026-08-17 12:00, see SetupTest) -- never against
// what the request says. Tuesday 2026-08-18 is open 08:00-20:00 with one place by default.

// hourlyInput is a Tuesday booking from startHour for the given length.
func (s *serviceSuite) hourlyInput(day int, startHour float64, length time.Duration) entity.CreateBookingInput {
	start := s.localDate(day).Add(time.Duration(startHour * float64(time.Hour)))
	return entity.CreateBookingInput{
		ParkingSpotID: s.parkingSpotID,
		UserID:        "11111111-1111-1111-1111-111111111111",
		Start:         start,
		End:           start.Add(length),
	}
}

// expectAccepted lets a valid hourly booking through pricing and creation.
func (s *serviceSuite) expectAccepted() {
	s.parkingClient.EXPECT().GetHourlyPrice(gomock.Any(), s.parkingSpotID, "").Return(100.0, nil)
	s.commissionRates.EXPECT().GetCommissionRate(gomock.Any()).Return(entity.PlatformSettings{}, nil)
	s.bookingRepository.EXPECT().GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, gomock.Any(), gomock.Any()).Return(nil, nil)
	s.bookingRepository.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			booking.ID = "created-id"
			return booking, nil
		})
}

func (s *serviceSuite) TestRules_TheRequestsScheduleIsIgnored() {
	// The request claims the spot is open around the clock; the spot itself closes at 20:00.
	input := s.hourlyInput(18, 21, 2*time.Hour)
	input.ParkingSpotSchedule = []entity.ScheduleEntry{{DayOfWeek: 1, StartHour: 0, EndHour: 24}}

	_, err := s.service.CreateBooking(s.context, input)

	s.Equal("OUTSIDE_PUBLISHED_HOURS", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestRules_TheRequestsCapacityIsIgnored() {
	// The request claims five places; the spot has one, and it's taken.
	input := s.hourlyInput(18, 10, time.Hour)
	input.Capacity = 5
	s.parkingClient.EXPECT().GetHourlyPrice(gomock.Any(), s.parkingSpotID, "").Return(100.0, nil)
	s.commissionRates.EXPECT().GetCommissionRate(gomock.Any()).Return(entity.PlatformSettings{}, nil)
	s.bookingRepository.EXPECT().GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, gomock.Any(), gomock.Any()).
		Return([]entity.TimeWindow{{Start: input.Start.UTC(), End: input.End.UTC()}}, nil)

	_, err := s.service.CreateBooking(s.context, input)

	s.True(errors.IsConflict(err))
}

func (s *serviceSuite) TestRules_ABookingAlreadyGoneIsRefused() {
	// 08:00-10:00 on the 17th, while it's 12:00.
	_, err := s.service.CreateBooking(s.context, s.hourlyInput(17, 8, 2*time.Hour))

	s.Equal("BOOKING_IN_PAST", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestRules_AStartAFewMinutesAgoIsStillFine() {
	// The driver picked 12:00 and paid at 12:10 (on a Monday the spot is open, for this test).
	s.spotRules.Schedule = []entity.ScheduleEntry{{DayOfWeek: 0, StartHour: 8, EndHour: 20}}
	s.expectAccepted()

	got, err := s.service.CreateBooking(s.context, s.hourlyInput(17, 11.84, time.Hour))

	s.Require().NoError(err)
	s.Equal("created-id", got.ID)
}

func (s *serviceSuite) TestRules_TooShortIsRefused() {
	_, err := s.service.CreateBooking(s.context, s.hourlyInput(18, 10, 20*time.Minute))

	s.Equal("BOOKING_TOO_SHORT", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestRules_ThirtyMinutesIsFine() {
	s.expectAccepted()

	_, err := s.service.CreateBooking(s.context, s.hourlyInput(18, 10, 30*time.Minute))

	s.Require().NoError(err)
}

func (s *serviceSuite) TestRules_TooLongIsRefused() {
	s.spotRules.Schedule = allDayEveryDay()

	_, err := s.service.CreateBooking(s.context, s.hourlyInput(18, 10, 32*24*time.Hour))

	s.Equal("BOOKING_TOO_LONG", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestRules_ASpotThatDoesntTakeHourlyBookingsRefusesThem() {
	s.spotRules.HourlyEnabled = false

	_, err := s.service.CreateBooking(s.context, s.hourlyInput(18, 10, time.Hour))

	s.Equal("HOURLY_NOT_OFFERED", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestRules_TouchingWindowsCountAsOneStretch() {
	// Open 08-12 and 12-20: 11:00 to 13:00 is open the whole time.
	s.spotRules.Schedule = []entity.ScheduleEntry{
		{DayOfWeek: 1, StartHour: 8, EndHour: 12},
		{DayOfWeek: 1, StartHour: 12, EndHour: 20},
	}
	s.expectAccepted()

	_, err := s.service.CreateBooking(s.context, s.hourlyInput(18, 11, 2*time.Hour))

	s.Require().NoError(err)
}

func (s *serviceSuite) TestRules_AGapBetweenWindowsStillRefuses() {
	// Open 08-12 and 14-20: 11:00 to 15:00 runs into the closed hours.
	s.spotRules.Schedule = []entity.ScheduleEntry{
		{DayOfWeek: 1, StartHour: 8, EndHour: 12},
		{DayOfWeek: 1, StartHour: 14, EndHour: 20},
	}

	_, err := s.service.CreateBooking(s.context, s.hourlyInput(18, 11, 4*time.Hour))

	s.Equal("OUTSIDE_PUBLISHED_HOURS", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestRules_MoreThanAYearAheadIsRefused_AnyKind() {
	s.spotRules.Schedule = allDayEveryDay()
	farAway := s.clock.AddDate(1, 0, 2)
	hourly := s.hourlyInput(18, 10, time.Hour)
	hourly.Start, hourly.End = farAway, farAway.Add(time.Hour)

	_, err := s.service.CreateBooking(s.context, hourly)
	s.Equal("BOOKING_TOO_FAR", errors.InvalidInputCode(err))

	stay := entity.CreateBookingInput{
		ParkingSpotID: s.parkingSpotID, Start: farAway, End: farAway.Add(time.Hour),
		Kind: enum.BookingKindStay, StayEndDate: s.dateStr(farAway.AddDate(0, 0, 2)),
	}
	// Refused before asking parking-service for any quote.
	_, err = s.service.CreateBooking(s.context, stay)
	s.Equal("BOOKING_TOO_FAR", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestRules_APausedSpotRefusesEveryKind() {
	s.spotRules.Active = false
	for _, kind := range []enum.BookingKind{enum.BookingKindHourly, enum.BookingKindStay, enum.BookingKindHalfDay} {
		input := s.hourlyInput(18, 10, time.Hour)
		input.Kind = kind
		input.StayEndDate = "2026-08-20"

		_, err := s.service.CreateBooking(s.context, input)

		s.Equal("SPOT_NOT_ACTIVE", errors.InvalidInputCode(err), kind.String())
	}
}

func allDayEveryDay() []entity.ScheduleEntry {
	schedule := make([]entity.ScheduleEntry, 7)
	for day := range schedule {
		schedule[day] = entity.ScheduleEntry{DayOfWeek: day, StartHour: 0, EndHour: 24}
	}
	return schedule
}
