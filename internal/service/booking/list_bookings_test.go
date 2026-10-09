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
	"github.com/example/bookings-service/internal/service/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(serviceSuite))
}

const testHoldDuration = 15 * time.Minute

type serviceSuite struct {
	suite.Suite
	controller        *gomock.Controller
	bookingRepository *mocks.MockBookingRepository
	parkingClient     *mocks.MockParkingClient
	commissionRates   *mocks.MockCommissionRateReader
	service           *booking.Service
	context           context.Context
	location          *time.Location
	parkingSpotID     string
	// spotOwnerID is who GetSpotOwnerID reports as the spot's owner -- someone other than
	// the drivers these tests book as, unless a test sets it to one of them.
	spotOwnerID string
	// vehicleRules is what GetVehicleRules reports the spot asks of a booking's vehicle --
	// nothing (a car-only spot where a vehicle is optional), unless a test sets it.
	vehicleRules entity.SpotVehicleRules
}

func (s *serviceSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookingRepository = mocks.NewMockBookingRepository(s.controller)
	s.parkingClient = mocks.NewMockParkingClient(s.controller)
	s.commissionRates = mocks.NewMockCommissionRateReader(s.controller)

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location

	s.service = booking.NewService(s.bookingRepository, s.parkingClient, s.commissionRates, location, testHoldDuration)
	s.context = context.Background()
	s.parkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.spotOwnerID = "99999999-9999-9999-9999-999999999999"
	s.parkingClient.EXPECT().GetSpotOwnerID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) (string, error) { return s.spotOwnerID, nil }).
		AnyTimes()
	s.vehicleRules = entity.SpotVehicleRules{AcceptedTypes: []string{"car"}}
	s.parkingClient.EXPECT().GetVehicleRules(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) (entity.SpotVehicleRules, error) { return s.vehicleRules, nil }).
		AnyTimes()
}

func (s *serviceSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *serviceSuite) localDate(day int) time.Time {
	return time.Date(2026, 8, day, 0, 0, 0, 0, s.location)
}

// futureDate returns local midnight `daysFromNow` days out from whenever the test
// actually runs -- unlike localDate's fixed 2026-08-* dates, it never rots into "today"
// (or the past) as the calendar advances, which matters for STAY tests: a quoted start
// that's already today or earlier gets clamped to "now" by CreateBooking (see
// TestCreateBooking_Stay_StartingTodayClampsToNow), which would break these otherwise.
func (s *serviceSuite) futureDate(daysFromNow int) time.Time {
	now := time.Now().In(s.location).AddDate(0, 0, daysFromNow)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
}

// dateStr formats t as the YYYY-MM-DD parking-service/booking-service exchange for a
// stay's start/end date, in the suite's location.
func (s *serviceSuite) dateStr(t time.Time) string {
	return t.In(s.location).Format("2006-01-02")
}

func (s *serviceSuite) TestListBookings_NoFiltersReturnsGroupedResult() {
	window := entity.BookingWindow{
		Start:  s.localDate(18).Add(10 * time.Hour).UTC(),
		End:    s.localDate(18).Add(11 * time.Hour).UTC(),
		Status: enum.BookingStatusConfirmed,
	}
	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, (*time.Time)(nil), (*time.Time)(nil), (*enum.BookingStatus)(nil)).
		Return([]entity.BookingWindow{window}, nil)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, nil, nil, nil, false)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal([]entity.BookingWindow{window}, got[0].Bookings)
}

func (s *serviceSuite) TestListBookings_PassesRangeAndStatusThrough() {
	from := s.localDate(18)
	to := s.localDate(20)
	status := enum.BookingStatusConfirmed

	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, &from, &to, &status).
		Return(nil, nil)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, &from, &to, &status, false)

	s.Require().NoError(err)
	s.Nil(got)
}

func (s *serviceSuite) TestListBookings_InvalidDateRange() {
	from := s.localDate(19)
	to := s.localDate(18)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, &from, &to, nil, false)

	s.Nil(got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("INVALID_DATE_RANGE", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestListBookings_OnlyFromGivenSkipsRangeValidation() {
	from := s.localDate(18)

	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, &from, (*time.Time)(nil), (*enum.BookingStatus)(nil)).
		Return(nil, nil)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, &from, nil, nil, false)

	s.Require().NoError(err)
	s.Nil(got)
}

func (s *serviceSuite) TestListBookings_RepositoryErrorPropagates() {
	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, (*time.Time)(nil), (*time.Time)(nil), (*enum.BookingStatus)(nil)).
		Return(nil, repositoryFailure)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, nil, nil, nil, false)

	s.Nil(got)
	s.ErrorIs(err, repositoryFailure)
}

func (s *serviceSuite) TestListBookings_ActiveOnlyExcludesInactiveAndExpiredHolds() {
	expiredHold := time.Now().Add(-time.Hour)
	start := s.localDate(18).Add(10 * time.Hour).UTC()
	end := s.localDate(18).Add(11 * time.Hour).UTC()
	confirmed := entity.BookingWindow{Start: start, End: end, Status: enum.BookingStatusConfirmed}
	inProgress := entity.BookingWindow{Start: start, End: end, Status: enum.BookingStatusInProgress}
	cancelled := entity.BookingWindow{Start: start, End: end, Status: enum.BookingStatusCancelled}
	expiredHoldBooking := entity.BookingWindow{Start: start, End: end, Status: enum.BookingStatusPendingPayment, HoldExpiresAt: &expiredHold}

	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, (*time.Time)(nil), (*time.Time)(nil), (*enum.BookingStatus)(nil)).
		Return([]entity.BookingWindow{confirmed, inProgress, cancelled, expiredHoldBooking}, nil)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, nil, nil, nil, true)

	s.Require().NoError(err)
	s.Require().Len(got, 1, "only the CONFIRMED and IN_PROGRESS bookings should be active")
	s.Require().Len(got[0].Bookings, 2)
	s.Equal(enum.BookingStatusConfirmed, got[0].Bookings[0].Status)
	s.Equal(enum.BookingStatusInProgress, got[0].Bookings[1].Status)
}

// The grace period only shows up in the occupancy view: active listings are padded on both
// sides and the requested range is widened to match, while the plain listing is untouched.
func (s *serviceSuite) TestListBookings_ActiveOnlyPadsWindowsAndWidensRangeByGracePeriod() {
	from := s.localDate(18)
	to := s.localDate(19)
	start := s.localDate(18).Add(10 * time.Hour).UTC()
	end := s.localDate(18).Add(11 * time.Hour).UTC()
	widenedFrom := from.Add(-entity.GracePeriod)
	widenedTo := to.Add(entity.GracePeriod)

	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, &widenedFrom, &widenedTo, (*enum.BookingStatus)(nil)).
		Return([]entity.BookingWindow{{Start: start, End: end, Status: enum.BookingStatusConfirmed}}, nil)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, &from, &to, nil, true)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Require().Len(got[0].Bookings, 1)
	s.True(got[0].Bookings[0].Start.Equal(start.Add(-entity.GracePeriod)))
	s.True(got[0].Bookings[0].End.Equal(end.Add(entity.GracePeriod)))
}

func (s *serviceSuite) TestListBookings_NotActiveOnlyKeepsRealWindows() {
	from := s.localDate(18)
	to := s.localDate(19)
	start := s.localDate(18).Add(10 * time.Hour).UTC()
	end := s.localDate(18).Add(11 * time.Hour).UTC()
	window := entity.BookingWindow{Start: start, End: end, Status: enum.BookingStatusConfirmed}

	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, &from, &to, (*enum.BookingStatus)(nil)).
		Return([]entity.BookingWindow{window}, nil)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, &from, &to, nil, false)

	s.Require().NoError(err)
	s.Equal([]entity.BookingWindow{window}, got[0].Bookings)
}

var repositoryFailure = repositoryFailureFixture{}

type repositoryFailureFixture struct{}

func (repositoryFailureFixture) Error() string { return "connection refused" }
