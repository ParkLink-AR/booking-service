package booking_test

import (
	"time"

	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

const otherParkingSpotID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

// caps turns spot ids into capacity-1 SpotCapacity entries (the common case).
func caps(ids ...string) []entity.SpotCapacity {
	out := make([]entity.SpotCapacity, len(ids))
	for i, id := range ids {
		out[i] = entity.SpotCapacity{ID: id, Capacity: 1}
	}
	return out
}

func (s *serviceSuite) TestGetBookableSpots_NoBookingsAllAreBookable() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	spotIDs := []string{s.parkingSpotID, otherParkingSpotID}

	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), spotIDs, start.UTC(), end.UTC()).
		Return(nil, nil)

	got, err := s.service.GetBookableSpots(s.context, caps(spotIDs...), start, end)

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

	got, err := s.service.GetBookableSpots(s.context, caps(spotIDs...), start, end)

	s.Require().NoError(err)
	s.Equal([]string{otherParkingSpotID}, got)
}

func (s *serviceSuite) TestGetBookableSpots_CapacityKeepsPlayaBookableUntilFull() {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	spots := []entity.SpotCapacity{{ID: s.parkingSpotID, Capacity: 2}}

	// One active booking, capacity 2 -> still has room.
	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), []string{s.parkingSpotID}, start.UTC(), end.UTC()).
		Return([]entity.SpotBooking{
			{ParkingSpotID: s.parkingSpotID, Status: enum.BookingStatusConfirmed},
		}, nil)

	got, err := s.service.GetBookableSpots(s.context, spots, start, end)
	s.Require().NoError(err)
	s.Equal([]string{s.parkingSpotID}, got)

	// Two active bookings, capacity 2 -> full.
	s.bookingRepository.EXPECT().
		GetBookingsForSpotsInRange(gomock.Any(), []string{s.parkingSpotID}, start.UTC(), end.UTC()).
		Return([]entity.SpotBooking{
			{ParkingSpotID: s.parkingSpotID, Status: enum.BookingStatusConfirmed},
			{ParkingSpotID: s.parkingSpotID, Status: enum.BookingStatusConfirmed},
		}, nil)

	got, err = s.service.GetBookableSpots(s.context, spots, start, end)
	s.Require().NoError(err)
	s.Empty(got)
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

	got, err := s.service.GetBookableSpots(s.context, caps(spotIDs...), start, end)

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

	got, err := s.service.GetBookableSpots(s.context, caps(spotIDs...), start, end)

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

		got, err := s.service.GetBookableSpots(s.context, caps(spotIDs...), start, end)

		s.Require().NoError(err)
		s.Equal(spotIDs, got, "status %s must not exclude the spot", status)
	}
}

func (s *serviceSuite) TestGetBookableSpots_InvalidDateRange() {
	start := time.Date(2026, 8, 24, 11, 0, 0, 0, s.location)
	end := time.Date(2026, 8, 24, 10, 0, 0, 0, s.location)

	got, err := s.service.GetBookableSpots(s.context, caps(s.parkingSpotID), start, end)

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

	got, err := s.service.GetBookableSpots(s.context, caps(spotIDs...), start, end)

	s.Nil(got)
	s.ErrorIs(err, repositoryFailure)
}
