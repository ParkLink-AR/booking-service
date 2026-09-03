package booking_test

import (
	"time"

	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// tuesdaySchedule is the published schedule for 2026-08-18 (a Tuesday, dayOfWeek=1),
// open 08:00-20:00.
var tuesdaySchedule = []entity.ScheduleEntry{{DayOfWeek: 1, StartHour: 8, EndHour: 20}}

func (s *serviceSuite) TestCreateBooking_Success() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		UserID:              "11111111-1111-1111-1111-111111111111",
		Start:               start,
		End:                 end,
		Amount:              5000,
	}

	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return(nil, nil)
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			s.Equal(input.ParkingSpotID, booking.ParkingSpotID)
			s.Equal(input.UserID, booking.UserID)
			s.True(booking.Start.Equal(start.UTC()))
			s.True(booking.End.Equal(end.UTC()))
			s.Equal(enum.BookingStatusPendingPayment, booking.Status)
			s.Require().NotNil(booking.HoldExpiresAt)
			s.WithinDuration(time.Now().Add(testHoldDuration), *booking.HoldExpiresAt, time.Second)
			s.Equal(5000, booking.Amount)
			booking.ID = "created-id"
			return booking, nil
		})

	got, err := s.service.CreateBooking(s.context, input)

	s.Require().NoError(err)
	s.Equal("created-id", got.ID)
}

func (s *serviceSuite) TestCreateBooking_InvalidDateRange() {
	start := s.localDate(18).Add(11 * time.Hour)
	end := s.localDate(18).Add(10 * time.Hour)
	input := entity.CreateBookingInput{ParkingSpotID: s.parkingSpotID, Start: start, End: end}

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("INVALID_DATE_RANGE", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestCreateBooking_OutsidePublishedHours() {
	start := s.localDate(18).Add(21 * time.Hour) // schedule closes at 20:00
	end := s.localDate(18).Add(22 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
	}

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("OUTSIDE_PUBLISHED_HOURS", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestCreateBooking_NoScheduleProvidedIsOutsidePublishedHours() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{ParkingSpotID: s.parkingSpotID, Start: start, End: end}

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("OUTSIDE_PUBLISHED_HOURS", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestCreateBooking_OverlapWithActiveBookingIsConflict() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
	}

	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return([]entity.TimeWindow{{Start: start.UTC(), End: end.UTC()}}, nil)

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsConflict(err))
}

func (s *serviceSuite) TestCreateBooking_CapacityAllowsConcurrentUpToLimit() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		UserID:              "11111111-1111-1111-1111-111111111111",
		Start:               start,
		End:                 end,
		Amount:              5000,
		Capacity:            2,
	}

	// One active booking overlapping the whole range; capacity 2 -> still fits.
	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return([]entity.TimeWindow{{Start: start.UTC(), End: end.UTC()}}, nil)
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			s.Equal(2, booking.Capacity)
			booking.ID = "created-id"
			return booking, nil
		})

	got, err := s.service.CreateBooking(s.context, input)

	s.Require().NoError(err)
	s.Equal("created-id", got.ID)
}

func (s *serviceSuite) TestCreateBooking_CapacityRejectsWhenAllPlacesConcurrentlyBooked() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
		Capacity:            2,
	}

	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return([]entity.TimeWindow{
			{Start: start.UTC(), End: end.UTC()},
			{Start: start.UTC(), End: end.UTC()},
		}, nil)

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsConflict(err))
}

func (s *serviceSuite) TestCreateBooking_SequentialBookingsDoNotCountAsConcurrent() {
	start := s.localDate(18).Add(8 * time.Hour)
	end := s.localDate(18).Add(12 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		UserID:              "11111111-1111-1111-1111-111111111111",
		Start:               start,
		End:                 end,
		Amount:              5000,
		Capacity:            2,
	}

	// Three back-to-back bookings inside the requested range -> never more than 1 concurrent.
	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return([]entity.TimeWindow{
			{Start: s.localDate(18).Add(8 * time.Hour).UTC(), End: s.localDate(18).Add(9 * time.Hour).UTC()},
			{Start: s.localDate(18).Add(9 * time.Hour).UTC(), End: s.localDate(18).Add(10 * time.Hour).UTC()},
			{Start: s.localDate(18).Add(10 * time.Hour).UTC(), End: s.localDate(18).Add(11 * time.Hour).UTC()},
		}, nil)
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			booking.ID = "created-id"
			return booking, nil
		})

	got, err := s.service.CreateBooking(s.context, input)

	s.Require().NoError(err)
	s.Equal("created-id", got.ID)
}

func (s *serviceSuite) TestCreateBooking_RepositoryCreateErrorPropagates() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
	}

	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return(nil, nil)
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		Return(entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking"))

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsConflict(err))
}
