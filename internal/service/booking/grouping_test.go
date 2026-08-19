package booking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

func TestGroupingSuite(t *testing.T) {
	suite.Run(t, new(groupingSuite))
}

type groupingSuite struct {
	suite.Suite
	location *time.Location
}

func (s *groupingSuite) SetupTest() {
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location
}

func (s *groupingSuite) localTime(day, hour int) time.Time {
	return time.Date(2026, 8, day, hour, 0, 0, 0, s.location)
}

func (s *groupingSuite) TestGroupByDay_EmptyInputReturnsNil() {
	got := groupByDay(nil, s.location)
	s.Nil(got)
}

func (s *groupingSuite) TestGroupByDay_SingleDaySingleBooking() {
	window := entity.BookingWindow{
		Start:  s.localTime(18, 10).UTC(),
		End:    s.localTime(18, 11).UTC(),
		Status: enum.BookingStatusConfirmed,
	}

	got := groupByDay([]entity.BookingWindow{window}, s.location)

	s.Require().Len(got, 1)
	s.True(got[0].Date.Equal(s.localTime(18, 0)))
	s.Equal([]entity.BookingWindow{window}, got[0].Bookings)
}

func (s *groupingSuite) TestGroupByDay_ConsecutiveBookingsSameDayAreGrouped() {
	first := entity.BookingWindow{Start: s.localTime(18, 8).UTC(), End: s.localTime(18, 9).UTC(), Status: enum.BookingStatusConfirmed}
	second := entity.BookingWindow{Start: s.localTime(18, 14).UTC(), End: s.localTime(18, 15).UTC(), Status: enum.BookingStatusPendingPayment}

	got := groupByDay([]entity.BookingWindow{first, second}, s.location)

	s.Require().Len(got, 1)
	s.Equal([]entity.BookingWindow{first, second}, got[0].Bookings)
}

func (s *groupingSuite) TestGroupByDay_BookingsAcrossMultipleDaysProduceMultipleGroups() {
	day18 := entity.BookingWindow{Start: s.localTime(18, 8).UTC(), End: s.localTime(18, 9).UTC(), Status: enum.BookingStatusConfirmed}
	day19 := entity.BookingWindow{Start: s.localTime(19, 8).UTC(), End: s.localTime(19, 9).UTC(), Status: enum.BookingStatusConfirmed}

	got := groupByDay([]entity.BookingWindow{day18, day19}, s.location)

	s.Require().Len(got, 2)
	s.True(got[0].Date.Equal(s.localTime(18, 0)))
	s.Equal([]entity.BookingWindow{day18}, got[0].Bookings)
	s.True(got[1].Date.Equal(s.localTime(19, 0)))
	s.Equal([]entity.BookingWindow{day19}, got[1].Bookings)
}

// A booking starting just after local midnight belongs to the new day even though its
// UTC instant is still within the previous UTC calendar day (Buenos Aires is UTC-3).
func (s *groupingSuite) TestGroupByDay_UsesLocalCalendarDayNotUTC() {
	justAfterMidnightLocal := s.localTime(19, 0).Add(30 * time.Minute) // 2026-08-19T00:30 -03:00 == 2026-08-19T03:30 UTC
	window := entity.BookingWindow{Start: justAfterMidnightLocal.UTC(), End: justAfterMidnightLocal.Add(time.Hour).UTC(), Status: enum.BookingStatusConfirmed}

	got := groupByDay([]entity.BookingWindow{window}, s.location)

	s.Require().Len(got, 1)
	s.True(got[0].Date.Equal(s.localTime(19, 0)))
}
