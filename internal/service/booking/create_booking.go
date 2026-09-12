package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateBooking validates input.Start < input.End, then branches on input.Kind:
//
//   - HOURLY confirms the requested range falls within the parking spot's published hours
//     (input.ParkingSpotSchedule, provided by the caller -- booking-service does not fetch
//     this itself), same as before this Kind existed.
//   - STAY skips the published-hours check (it's meaningless for a multi-day range) and
//     instead asks parking-service for a quote (see priceStay): the spot's owner-configured
//     stay terms decide whether the requested start/end date and check-in hour is even
//     allowed, and the resulting exact start/end instants and amount -- input.End and
//     input.Amount are never trusted for a stay, so a modified client can't shrink the
//     range or substitute a cheaper price.
//
// Either way it then checks that fewer than the spot's capacity active bookings already
// overlap the (possibly server-recomputed) range, and creates the booking as
// PENDING_PAYMENT with a hold that expires after the service's holdDuration. Both kinds
// share the same bookings table and time_range column, so an hourly and a stay booking on
// the same spot correctly block each other with no extra logic.
//
// The concurrency check here is a pre-check for a fast, friendly conflict response; the
// repository re-checks it under a per-spot advisory lock and is the final authority,
// mapping a lost race to the same conflict error.
func (service *Service) CreateBooking(ctx context.Context, input entity.CreateBookingInput) (entity.Booking, error) {
	if !input.Start.Before(input.End) {
		return entity.Booking{}, errors.NewInvalidInput("INVALID_DATE_RANGE", "start must be before end")
	}

	start, end, amount := input.Start, input.End, input.Amount

	if input.Kind == enum.BookingKindStay {
		quote, err := service.priceStay(ctx, input)
		if err != nil {
			return entity.Booking{}, err
		}
		start, end, amount = quote.Start, quote.End, quote.Amount
		// A stay whose check-in instant already passed by the time this
		// request lands (e.g. driver picked "today at 09:00" and it's now
		// 09:03) still needs to book from right now, not from the already-past
		// instant -- clamping keeps a booking that already ended earlier
		// today (still occupying the [checkin, now) sliver in the overlap
		// check below) from registering as a false conflict against a stay
		// that only needs the spot from this moment on. Same boundary
		// parking-service's StaysService.assertRangeFree checks against when
		// quoting.
		if now := time.Now(); start.Before(now) {
			start = now
		}
	} else if !isWithinPublishedHours(input.Start, input.End, input.ParkingSpotSchedule, service.location) {
		return entity.Booking{}, errors.NewInvalidInput("OUTSIDE_PUBLISHED_HOURS",
			"requested time is outside the parking spot's published hours")
	}

	capacity := input.Capacity
	if capacity < 1 {
		capacity = 1
	}

	activeBookings, err := service.bookingRepository.GetActiveBookingsInRange(ctx, input.ParkingSpotID, start.UTC(), end.UTC())
	if err != nil {
		return entity.Booking{}, err
	}
	if entity.MaxConcurrency(activeBookings, start.UTC(), end.UTC()) >= capacity {
		return entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking")
	}

	holdExpiresAt := time.Now().Add(service.holdDuration)
	booking := entity.Booking{
		ParkingSpotID: input.ParkingSpotID,
		UserID:        input.UserID,
		Start:         start.UTC(),
		End:           end.UTC(),
		Status:        enum.BookingStatusPendingPayment,
		HoldExpiresAt: &holdExpiresAt,
		Amount:        amount,
		Capacity:      capacity,
		Kind:          input.Kind,
	}

	return service.bookingRepository.CreateBooking(ctx, booking)
}

type stayQuote struct {
	Start  time.Time
	End    time.Time
	Amount int
}

// priceStay resolves a STAY booking's authoritative range and amount by asking
// parking-service to quote it -- see parkingclient.Client.GetStayQuote. input.Start is the
// driver's chosen check-in instant: its calendar date and hour (both in the service's
// location) are what "start date"/"check-in time" mean to the driver. input.StayEndDate is
// the calendar date they picked to check out -- parking-service applies the same check-in
// hour to it (a stay runs in 24h periods from check-in, not midnight to midnight) and
// returns the exact start/end instants and amount, never trusted from input.
func (service *Service) priceStay(ctx context.Context, input entity.CreateBookingInput) (stayQuote, error) {
	localStart := input.Start.In(service.location)
	startDate := localStart.Format("2006-01-02")
	startHour := localStart.Hour()

	quote, err := service.parkingClient.GetStayQuote(ctx, input.ParkingSpotID, startDate, input.StayEndDate, startHour)
	if err != nil {
		return stayQuote{}, err
	}

	start, err := time.Parse(time.RFC3339, quote.StartDateTime)
	if err != nil {
		return stayQuote{}, errors.NewUpstream("priceStay", err)
	}
	end, err := time.Parse(time.RFC3339, quote.EndDateTime)
	if err != nil {
		return stayQuote{}, errors.NewUpstream("priceStay", err)
	}

	return stayQuote{Start: start, End: end, Amount: quote.Amount}, nil
}
