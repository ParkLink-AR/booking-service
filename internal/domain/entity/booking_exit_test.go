package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

func TestBookingExitSuite(t *testing.T) {
	suite.Run(t, new(bookingExitSuite))
}

type bookingExitSuite struct {
	suite.Suite
}

var exitBookingStart = time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)

// exitBooking is a two-hour booking, 18:00-20:00, paid $2.400 by the driver ($2.000 the owner's
// own price plus the platform's 20%).
func exitBooking() entity.Booking {
	ownerAmount := 200000
	return entity.Booking{
		Start:       exitBookingStart,
		End:         exitBookingStart.Add(2 * time.Hour),
		Amount:      240000,
		OwnerAmount: &ownerAmount,
		Kind:        enum.BookingKindHourly,
	}
}

var computeOverstayCases = []struct {
	name   string
	stayed time.Duration // past the end
	want   entity.Overstay
}{
	{"left before the end", -30 * time.Minute, entity.Overstay{}},
	{"left exactly at the end", 0, entity.Overstay{}},
	{"stayed within the grace period", 10 * time.Minute, entity.Overstay{Minutes: 10}},
	{"stayed exactly the grace period", 15 * time.Minute, entity.Overstay{Minutes: 15}},
	{"just past the grace period is charged from the end", 16 * time.Minute,
		entity.Overstay{Minutes: 16, ChargedMinutes: 16, Amount: 32000, OwnerAmount: 26667}},
	{"twenty minutes extra are all charged", 20 * time.Minute,
		entity.Overstay{Minutes: 20, ChargedMinutes: 20, Amount: 40000, OwnerAmount: 33333}},
	{"half an hour is half the hourly rate", 30 * time.Minute,
		entity.Overstay{Minutes: 30, ChargedMinutes: 30, Amount: 60000, OwnerAmount: 50000}},
	{"a partial minute counts as a minute", 20*time.Minute + 5*time.Second,
		entity.Overstay{Minutes: 21, ChargedMinutes: 21, Amount: 42000, OwnerAmount: 35000}},
	{"five hours extra", 5 * time.Hour,
		entity.Overstay{Minutes: 300, ChargedMinutes: 300, Amount: 600000, OwnerAmount: 500000}},
}

func (s *bookingExitSuite) TestComputeOverstay() {
	booking := exitBooking()
	for _, testCase := range computeOverstayCases {
		got := entity.ComputeOverstay(booking, booking.End.Add(testCase.stayed))
		s.Equal(testCase.want, got, testCase.name)
	}
}

func (s *bookingExitSuite) TestComputeOverstay_OwnerAmountMissingChargesTheDriverOnly() {
	booking := exitBooking()
	booking.OwnerAmount = nil

	got := entity.ComputeOverstay(booking, booking.End.Add(30*time.Minute))

	s.Equal(entity.Overstay{Minutes: 30, ChargedMinutes: 30, Amount: 60000}, got)
}

func (s *bookingExitSuite) TestComputeOverstay_EmptyBookingChargesNothing() {
	booking := exitBooking()
	booking.End = booking.Start

	got := entity.ComputeOverstay(booking, booking.End.Add(time.Hour))

	s.Equal(entity.Overstay{Minutes: 60}, got)
}

var awaitingExitCases = []struct {
	name     string
	kind     enum.BookingKind
	status   enum.BookingStatus
	verified bool
	want     bool
}{
	{"verified and in progress", enum.BookingKindHourly, enum.BookingStatusInProgress, true, true},
	{"verified extension, still confirmed", enum.BookingKindHourly, enum.BookingStatusConfirmed, true, true},
	{"never verified", enum.BookingKindHourly, enum.BookingStatusConfirmed, false, false},
	{"already completed", enum.BookingKindHourly, enum.BookingStatusCompleted, true, false},
	{"a stay is not waited for", enum.BookingKindStay, enum.BookingStatusInProgress, true, false},
	{"a verified half-day is waited for, like an hourly booking", enum.BookingKindHalfDay, enum.BookingStatusInProgress, true, true},
	{"a half-day never verified completes on its own", enum.BookingKindHalfDay, enum.BookingStatusConfirmed, false, false},
	{"cancelled", enum.BookingKindHourly, enum.BookingStatusCancelled, true, false},
}

func (s *bookingExitSuite) TestAwaitingExit() {
	for _, testCase := range awaitingExitCases {
		booking := exitBooking()
		booking.Kind = testCase.kind
		booking.Status = testCase.status
		if testCase.verified {
			verifiedAt := exitBookingStart
			booking.VerifiedAt = &verifiedAt
		}
		s.Equal(testCase.want, booking.AwaitingExit(), testCase.name)
	}
}
