package booking_test

import (
	"time"

	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

func (s *serviceSuite) publishedHoursBase(day time.Time) *entity.AvailabilityBase {
	return &entity.AvailabilityBase{Days: []entity.DayBase{
		{
			Date: day,
			Windows: []entity.TimeWindow{
				{Start: day.Add(8 * time.Hour), End: day.Add(20 * time.Hour)},
			},
		},
	}}
}

func (s *serviceSuite) TestCreateBooking_Success() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID: s.parkingSpotID,
		UserID:        "11111111-1111-1111-1111-111111111111",
		Start:         start,
		End:           end,
		Amount:        5000,
	}

	s.parkingAvailability.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, start, end).
		Return(s.publishedHoursBase(s.localDate(18)), nil)
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
	start := s.localDate(18).Add(21 * time.Hour) // published base closes at 20:00
	end := s.localDate(18).Add(22 * time.Hour)
	input := entity.CreateBookingInput{ParkingSpotID: s.parkingSpotID, Start: start, End: end}

	s.parkingAvailability.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, start, end).
		Return(s.publishedHoursBase(s.localDate(18)), nil)

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("OUTSIDE_PUBLISHED_HOURS", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestCreateBooking_ParkingServiceErrorWrapsAsUpstream() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{ParkingSpotID: s.parkingSpotID, Start: start, End: end}

	s.parkingAvailability.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, start, end).
		Return(nil, repositoryFailure)

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsUpstream(err))
}

func (s *serviceSuite) TestCreateBooking_OverlapWithActiveBookingIsConflict() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{ParkingSpotID: s.parkingSpotID, Start: start, End: end}

	s.parkingAvailability.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, start, end).
		Return(s.publishedHoursBase(s.localDate(18)), nil)
	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return([]entity.TimeWindow{{Start: start.UTC(), End: end.UTC()}}, nil)

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsConflict(err))
}

func (s *serviceSuite) TestCreateBooking_RepositoryCreateErrorPropagates() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{ParkingSpotID: s.parkingSpotID, Start: start, End: end}

	s.parkingAvailability.EXPECT().
		GetAvailabilityBase(gomock.Any(), s.parkingSpotID, start, end).
		Return(s.publishedHoursBase(s.localDate(18)), nil)
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
