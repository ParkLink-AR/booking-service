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
	service           *booking.Service
	context           context.Context
	location          *time.Location
	parkingSpotID     string
}

func (s *serviceSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookingRepository = mocks.NewMockBookingRepository(s.controller)

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location

	s.service = booking.NewService(s.bookingRepository, location, testHoldDuration)
	s.context = context.Background()
	s.parkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *serviceSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *serviceSuite) localDate(day int) time.Time {
	return time.Date(2026, 8, day, 0, 0, 0, 0, s.location)
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
	confirmed := entity.BookingWindow{Status: enum.BookingStatusConfirmed}
	cancelled := entity.BookingWindow{Status: enum.BookingStatusCancelled}
	expiredHoldBooking := entity.BookingWindow{Status: enum.BookingStatusPendingPayment, HoldExpiresAt: &expiredHold}

	s.bookingRepository.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, (*time.Time)(nil), (*time.Time)(nil), (*enum.BookingStatus)(nil)).
		Return([]entity.BookingWindow{confirmed, cancelled, expiredHoldBooking}, nil)

	got, err := s.service.ListBookings(s.context, s.parkingSpotID, nil, nil, nil, true)

	s.Require().NoError(err)
	s.Require().Len(got, 1, "only the CONFIRMED booking should be active")
	s.Equal([]entity.BookingWindow{confirmed}, got[0].Bookings)
}

var repositoryFailure = repositoryFailureFixture{}

type repositoryFailureFixture struct{}

func (repositoryFailureFixture) Error() string { return "connection refused" }
