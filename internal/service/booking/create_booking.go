package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateBooking validates input.Start < input.End, confirms the requested range falls
// within the parking spot's published hours (input.ParkingSpotSchedule, provided by the
// caller -- booking-service does not fetch this itself), checks that fewer than the spot's
// capacity active bookings already overlap the requested range, and creates the booking as
// PENDING_PAYMENT with a hold that expires after the service's holdDuration.
//
// The concurrency check here is a pre-check for a fast, friendly conflict response; the
// repository re-checks it under a per-spot advisory lock and is the final authority,
// mapping a lost race to the same conflict error.
func (service *Service) CreateBooking(ctx context.Context, input entity.CreateBookingInput) (entity.Booking, error) {
	if !input.Start.Before(input.End) {
		return entity.Booking{}, errors.NewInvalidInput("INVALID_DATE_RANGE", "start must be before end")
	}

	if !isWithinPublishedHours(input.Start, input.End, input.ParkingSpotSchedule, service.location) {
		return entity.Booking{}, errors.NewInvalidInput("OUTSIDE_PUBLISHED_HOURS",
			"requested time is outside the parking spot's published hours")
	}

	capacity := input.Capacity
	if capacity < 1 {
		capacity = 1
	}

	activeBookings, err := service.bookingRepository.GetActiveBookingsInRange(ctx, input.ParkingSpotID, input.Start.UTC(), input.End.UTC())
	if err != nil {
		return entity.Booking{}, err
	}
	if entity.MaxConcurrency(activeBookings, input.Start.UTC(), input.End.UTC()) >= capacity {
		return entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking")
	}

	holdExpiresAt := time.Now().Add(service.holdDuration)
	booking := entity.Booking{
		ParkingSpotID: input.ParkingSpotID,
		UserID:        input.UserID,
		Start:         input.Start.UTC(),
		End:           input.End.UTC(),
		Status:        enum.BookingStatusPendingPayment,
		HoldExpiresAt: &holdExpiresAt,
		Amount:        input.Amount,
		Capacity:      capacity,
	}

	return service.bookingRepository.CreateBooking(ctx, booking)
}
