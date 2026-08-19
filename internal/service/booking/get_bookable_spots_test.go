package booking_test

import (
	"time"

	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

const otherParkingSpotID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func (s *serviceSuite) TestGetBookableSpots_NoBookingsAllAreBookable() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	spotIDs := []string{s.parkingSpotID, otherParkingSpotID}

	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), spotIDs, start.UTC(), end.UTC()).
		Return(nil, nil)

	got, err := s.service.GetBookableSpots(s.context, spotIDs, start, end)

	s.Require().NoError(err)
	s.Equal(spotIDs, got)
}

func (s *serviceSuite) TestGetBookableSpots_ConfirmedBookingExcludesSpot() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	spotIDs := []string{s.parkingSpotID, otherParkingSpotID}

	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), spotIDs, start.UTC(), end.UTC()).
		Return([]entity.SpotBooking{
			{ParkingSpotID: s.parkingSpotID, Status: enum.BookingStatusConfirmed},
		}, nil)

	got, err := s.service.GetBookableSpots(s.context, spotIDs, start, end)

	s.Require().NoError(err)
	s.Equal([]string{otherParkingSpotID}, got)
}

func (s *serviceSuite) TestGetBookableSpots_UnexpiredPendingPaymentExcludesSpot() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	futureHold := time.Now().Add(time.Hour)
	spotIDs := []string{s.parkingSpotID}

	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), spotIDs, start.UTC(), end.UTC()).
		Return([]entity.SpotBooking{
			{ParkingSpotID: s.parkingSpotID, Status: enum.BookingStatusPendingPayment, HoldExpiresAt: &futureHold},
		}, nil)

	got, err := s.service.GetBookableSpots(s.context, spotIDs, start, end)

	s.Require().NoError(err)
	s.Empty(got)
}

func (s *serviceSuite) TestGetBookableSpots_ExpiredPendingPaymentDoesNotExcludeSpot() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	expiredHold := time.Now().Add(-time.Hour)
	spotIDs := []string{s.parkingSpotID}

	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), spotIDs, start.UTC(), end.UTC()).
		Return([]entity.SpotBooking{
			{ParkingSpotID: s.parkingSpotID, Status: enum.BookingStatusPendingPayment, HoldExpiresAt: &expiredHold},
		}, nil)

	got, err := s.service.GetBookableSpots(s.context, spotIDs, start, end)

	s.Require().NoError(err)
	s.Equal(spotIDs, got)
}

var inactiveBookableStatusCases = []enum.BookingStatus{
	enum.BookingStatusCompleted,
	enum.BookingStatusCancelled,
	enum.BookingStatusExpired,
}

func (s *serviceSuite) TestGetBookableSpots_InactiveStatusesDoNotExcludeSpot() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	spotIDs := []string{s.parkingSpotID}

	for _, status := range inactiveBookableStatusCases {
		s.bookingRepository.EXPECT().
			GetBookingsForSpotsInRange(gomock.Any(), spotIDs, start.UTC(), end.UTC()).
			Return([]entity.SpotBooking{{ParkingSpotID: s.parkingSpotID, Status: status}}, nil)

		got, err := s.service.GetBookableSpots(s.context, spotIDs, start, end)

		s.Require().NoError(err)
		s.Equal(spotIDs, got, "status %s must not exclude the spot", status)
	}
}

func (s *serviceSuite) TestGetBookableSpots_InvalidDateRange() {
	start := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)

	got, err := s.service.GetBookableSpots(s.context, []string{s.parkingSpotID}, start, end)

	s.Nil(got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("INVALID_DATE_RANGE", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestGetBookableSpots_EmptySpotIDsSkipsRepository() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)

	got, err := s.service.GetBookableSpots(s.context, nil, start, end)

	s.Require().NoError(err)
	s.Nil(got)
}

func (s *serviceSuite) TestGetBookableSpots_RepositoryErrorPropagates() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	spotIDs := []string{s.parkingSpotID}

	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), spotIDs, start.UTC(), end.UTC()).
		Return(nil, repositoryFailure)

	got, err := s.service.GetBookableSpots(s.context, spotIDs, start, end)

	s.Nil(got)
	s.ErrorIs(err, repositoryFailure)
}
