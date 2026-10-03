package booking_test

import (
	"time"

	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/repository/parkingclient"
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

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	// $100/hour x 1 hour = 10000 centavos, +5% commission = 10500 -- the client's
	// (deliberately wrong) Amount: 5000 is never trusted for an HOURLY booking either,
	// same as a STAY's.
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(100.0, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
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
			s.Equal(10500, booking.Amount)
			s.Require().NotNil(booking.OwnerAmount)
			s.Equal(10000, *booking.OwnerAmount)
			booking.ID = "created-id"
			return booking, nil
		})

	got, err := s.service.CreateBooking(s.context, input)

	s.Require().NoError(err)
	s.Equal("created-id", got.ID)
}

// An optional vehicle snapshot, when the driver picked one, is copied onto the booking
// verbatim -- see Service.CreateBooking. A booking with no vehicle picked (the Success test
// above) leaves all four fields nil.
func (s *serviceSuite) TestCreateBooking_WithVehicle_CopiesSnapshotOntoBooking() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		UserID:              "11111111-1111-1111-1111-111111111111",
		Start:               start,
		End:                 end,
		Amount:              5000,
		VehiclePlate:        "AB123CD",
		VehicleBrand:        "Toyota",
		VehicleModel:        "Corolla",
		VehicleCountry:      "AR",
	}

	s.parkingClient.EXPECT().IsSpotActive(gomock.Any(), s.parkingSpotID).Return(true, nil)
	s.parkingClient.EXPECT().GetHourlyPrice(gomock.Any(), s.parkingSpotID).Return(100.0, nil)
	s.commissionRates.EXPECT().GetCommissionRate(gomock.Any()).Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return(nil, nil)
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			s.Require().NotNil(booking.VehiclePlate)
			s.Equal("AB123CD", *booking.VehiclePlate)
			s.Require().NotNil(booking.VehicleBrand)
			s.Equal("Toyota", *booking.VehicleBrand)
			s.Require().NotNil(booking.VehicleModel)
			s.Equal("Corolla", *booking.VehicleModel)
			s.Require().NotNil(booking.VehicleCountry)
			s.Equal("AR", *booking.VehicleCountry)
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

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("OUTSIDE_PUBLISHED_HOURS", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestCreateBooking_Hourly_RejectsWhenSpotNotActive() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
	}

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(false, nil)

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("SPOT_NOT_ACTIVE", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestCreateBooking_Hourly_PropagatesSpotActiveCheckError() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
	}

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(false, errors.NewUpstream("parkingclient.getSpot", errors.NewNotFound("parking_spot", s.parkingSpotID)))

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsUpstream(err))
}

// Mirrors TestCreateBooking_Stay_UsesParkingServiceQuoteNotClientInput: proves the service
// ignores the client's Amount for an HOURLY booking too and prices it itself from the
// spot's own hourly rate plus the platform's current commission.
func (s *serviceSuite) TestCreateBooking_Hourly_IgnoresClientAmountPricesFromOwnerRatePlusCommission() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(13 * time.Hour) // 3 hours
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
		Amount:              1, // deliberately wrong -- must be ignored
	}

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	// $200/hour x 3 hours = 60000 centavos, +10% commission = 66000.
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(200.0, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 1000}, nil)
	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, start.UTC(), end.UTC()).
		Return(nil, nil)
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			s.Equal(66000, booking.Amount)
			s.Require().NotNil(booking.OwnerAmount)
			s.Equal(60000, *booking.OwnerAmount)
			booking.ID = "created-id"
			return booking, nil
		})

	got, err := s.service.CreateBooking(s.context, input)

	s.Require().NoError(err)
	s.Equal("created-id", got.ID)
}

// A stay-only spot (no hourly rate configured) rejects an HOURLY booking attempt with
// whatever error parkingclient.GetHourlyPrice reports, same propagation pattern as
// TestCreateBooking_Stay_PropagatesParkingServiceRejection.
func (s *serviceSuite) TestCreateBooking_Hourly_PropagatesHourlyPriceLookupError() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{
		ParkingSpotID:       s.parkingSpotID,
		ParkingSpotSchedule: tuesdaySchedule,
		Start:               start,
		End:                 end,
	}

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(0.0, errors.NewInvalidInput("HOURLY_NOT_AVAILABLE", "this parking spot does not accept hourly bookings"))

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("HOURLY_NOT_AVAILABLE", errors.InvalidInputCode(err))
}

func (s *serviceSuite) TestCreateBooking_NoScheduleProvidedIsOutsidePublishedHours() {
	start := s.localDate(18).Add(10 * time.Hour)
	end := s.localDate(18).Add(11 * time.Hour)
	input := entity.CreateBookingInput{ParkingSpotID: s.parkingSpotID, Start: start, End: end}

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
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

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(100.0, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
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

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(100.0, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
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

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(100.0, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
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

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(100.0, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
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

func (s *serviceSuite) TestCreateBooking_Stay_UsesParkingServiceQuoteNotClientInput() {
	// Comfortably in the future (unlike a hardcoded date, this never rots into "today"
	// as the calendar advances, which would trip the now-clamping tested separately
	// below) -- the client's Start/End/Amount are deliberately "wrong" here (a single
	// day, a lowball amount) to prove the service ignores them for a STAY and uses
	// whatever parking-service's quote says instead.
	quotedStart := s.futureDate(30)
	quotedEnd := s.futureDate(44) // two weeks later
	input := entity.CreateBookingInput{
		ParkingSpotID: s.parkingSpotID,
		UserID:        "11111111-1111-1111-1111-111111111111",
		Start:         quotedStart,
		End:           quotedStart.Add(24 * time.Hour),
		Amount:        1,
		Kind:          enum.BookingKindStay,
		StayEndDate:   s.dateStr(quotedEnd),
	}

	s.parkingClient.EXPECT().
		GetStayQuote(gomock.Any(), s.parkingSpotID, s.dateStr(quotedStart), s.dateStr(quotedEnd), 0).
		Return(parkingclient.StayQuote{
			StartDate:     s.dateStr(quotedStart),
			EndDate:       s.dateStr(quotedEnd),
			StartDateTime: quotedStart.Format(time.RFC3339),
			EndDateTime:   quotedEnd.Format(time.RFC3339),
			Amount:        50000,
		}, nil)
	// Owner's quoted 50000 + 5% commission added on top for the driver = 52500.
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)

	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, quotedStart.UTC(), quotedEnd.UTC()).
		Return(nil, nil)
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			s.True(booking.Start.Equal(quotedStart.UTC()))
			s.True(booking.End.Equal(quotedEnd.UTC()))
			s.Equal(52500, booking.Amount)
			s.Require().NotNil(booking.OwnerAmount)
			s.Equal(50000, *booking.OwnerAmount)
			s.Equal(enum.BookingKindStay, booking.Kind)
			booking.ID = "created-id"
			return booking, nil
		})

	got, err := s.service.CreateBooking(s.context, input)

	s.Require().NoError(err)
	s.Equal("created-id", got.ID)
}

func (s *serviceSuite) TestCreateBooking_Stay_PropagatesParkingServiceRejection() {
	start := s.futureDate(30)
	end := s.futureDate(60)
	input := entity.CreateBookingInput{
		ParkingSpotID: s.parkingSpotID,
		Start:         start,
		End:           start.Add(24 * time.Hour),
		Kind:          enum.BookingKindStay,
		StayEndDate:   s.dateStr(end),
	}

	s.parkingClient.EXPECT().
		GetStayQuote(gomock.Any(), s.parkingSpotID, s.dateStr(start), s.dateStr(end), 0).
		Return(parkingclient.StayQuote{}, errors.NewInvalidInput("STAY_NOT_AVAILABLE", "this spot does not offer stays"))

	got, err := s.service.CreateBooking(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("STAY_NOT_AVAILABLE", errors.InvalidInputCode(err))
}

// A stay quoted for "today" starts at local midnight, which by the time anyone can
// actually book it has already passed -- CreateBooking clamps it to now instead, so an
// already-finished booking earlier today (still occupying the [midnight, now) sliver)
// can't register as a false conflict for a stay that only needs the spot from this
// moment on. See the comment in create_booking.go for the same reasoning
// parking-service's StaysService.assertRangeFree applies when quoting.
func (s *serviceSuite) TestCreateBooking_Stay_StartingTodayClampsToNow() {
	now := time.Now().In(s.location)
	// Check-in at today's midnight (hour 0) -- like the pre-hour-of-day version of this
	// test, this guarantees the quoted start instant is already in the past by the time
	// CreateBooking clamps it, without racing the clock at whatever hour the test happens
	// to run.
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	today := s.dateStr(now)
	quotedEnd := s.futureDate(1)
	input := entity.CreateBookingInput{
		ParkingSpotID: s.parkingSpotID,
		Start:         todayMidnight,
		End:           todayMidnight.Add(time.Hour),
		Kind:          enum.BookingKindStay,
		StayEndDate:   s.dateStr(quotedEnd),
	}

	s.parkingClient.EXPECT().
		GetStayQuote(gomock.Any(), s.parkingSpotID, today, s.dateStr(quotedEnd), 0).
		Return(parkingclient.StayQuote{
			StartDate:     today,
			EndDate:       s.dateStr(quotedEnd),
			StartDateTime: todayMidnight.Format(time.RFC3339),
			EndDateTime:   quotedEnd.Format(time.RFC3339),
			Amount:        8000,
		}, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)

	beforeCall := time.Now()
	s.bookingRepository.EXPECT().
		GetActiveBookingsInRange(gomock.Any(), s.parkingSpotID, gomock.Any(), quotedEnd.UTC()).
		DoAndReturn(func(_ any, _ string, start, _ time.Time) ([]entity.TimeWindow, error) {
			// Clamped to "now" at call time, not to today's midnight.
			s.False(start.Before(beforeCall.UTC()))
			return nil, nil
		})
	s.bookingRepository.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, booking entity.Booking) (entity.Booking, error) {
			s.False(booking.Start.Before(beforeCall.UTC()))
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

	s.parkingClient.EXPECT().
		IsSpotActive(gomock.Any(), s.parkingSpotID).
		Return(true, nil)
	s.parkingClient.EXPECT().
		GetHourlyPrice(gomock.Any(), s.parkingSpotID).
		Return(100.0, nil)
	s.commissionRates.EXPECT().
		GetCommissionRate(gomock.Any()).
		Return(entity.PlatformSettings{CommissionRateBP: 500}, nil)
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
