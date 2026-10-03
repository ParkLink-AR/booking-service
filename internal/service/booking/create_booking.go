package booking

import (
	"context"
	"math"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateBooking validates input.Start < input.End, then branches on input.Kind:
//
//   - HOURLY first confirms with parking-service that the spot is still active (a paused
//     or deleted spot must reject any new reservation, even though its already-existing
//     bookings keep working), then that the requested range falls within the parking
//     spot's published hours (input.ParkingSpotSchedule, provided by the caller --
//     booking-service does not fetch this itself), same as before this Kind existed. Its
//     Amount is priced the same way a STAY is (see priceHourly): never trusted from
//     input, so a modified client can't substitute a cheaper price or skip the platform
//     commission that gets added on top of the owner's own rate.
//   - STAY skips both of those checks and instead asks parking-service for a quote (see
//     priceStay): that lookup only ever finds an active spot, so a paused or deleted one
//     surfaces the same rejection there, and the spot's owner-configured stay terms decide
//     whether the requested start/end date and check-in hour is even allowed -- the
//     resulting exact start/end instants and amount -- input.End and input.Amount are never
//     trusted for a stay, so a modified client can't shrink the range or substitute a
//     cheaper price.
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

	start, end := input.Start, input.End
	var amount, ownerAmount int

	if input.Kind == enum.BookingKindStay {
		quote, err := service.priceStay(ctx, input)
		if err != nil {
			return entity.Booking{}, err
		}
		start, end, amount, ownerAmount = quote.Start, quote.End, quote.Amount, quote.OwnerAmount
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
	} else {
		active, err := service.parkingClient.IsSpotActive(ctx, input.ParkingSpotID)
		if err != nil {
			return entity.Booking{}, err
		}
		if !active {
			return entity.Booking{}, errors.NewInvalidInput("SPOT_NOT_ACTIVE",
				"this parking spot is not accepting new reservations")
		}
		if !isWithinPublishedHours(input.Start, input.End, input.ParkingSpotSchedule, service.location) {
			return entity.Booking{}, errors.NewInvalidInput("OUTSIDE_PUBLISHED_HOURS",
				"requested time is outside the parking spot's published hours")
		}

		priced, err := service.priceHourly(ctx, input)
		if err != nil {
			return entity.Booking{}, err
		}
		amount, ownerAmount = priced.TotalAmount, priced.OwnerAmount
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
		OwnerAmount:   &ownerAmount,
		Capacity:      capacity,
		Kind:          input.Kind,
	}
	// Entirely optional, and never partially set -- the controller only forwards a vehicle
	// when all four fields were sent together (see createBookingRequest.Vehicle).
	if input.VehiclePlate != "" {
		booking.VehiclePlate = &input.VehiclePlate
		booking.VehicleBrand = &input.VehicleBrand
		booking.VehicleModel = &input.VehicleModel
		booking.VehicleCountry = &input.VehicleCountry
	}

	return service.bookingRepository.CreateBooking(ctx, booking)
}

type stayQuote struct {
	Start  time.Time
	End    time.Time
	Amount int
	// OwnerAmount is quote.Amount before addCommission adds the platform's cut --
	// the owner's own rate, stored on the booking so a later commission-rate change
	// can never retroactively change what this booking pays out (see Booking.OwnerAmount).
	OwnerAmount int
}

// priceStay resolves a STAY booking's authoritative range and amount by asking
// parking-service to quote it -- see parkingclient.Client.GetStayQuote. input.Start is the
// driver's chosen check-in instant: its calendar date and hour (both in the service's
// location) are what "start date"/"check-in time" mean to the driver. input.StayEndDate is
// the calendar date they picked to check out -- parking-service applies the same check-in
// hour to it (a stay runs in 24h periods from check-in, not midnight to midnight) and
// returns the exact start/end instants and amount, never trusted from input. The quoted
// amount is the owner's own rate (what they're set up to receive in full) -- addCommission
// adds the platform's cut on top of it, same as priceHourly below.
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

	amount, err := service.addCommission(ctx, quote.Amount)
	if err != nil {
		return stayQuote{}, err
	}

	return stayQuote{Start: start, End: end, Amount: amount, OwnerAmount: quote.Amount}, nil
}

// hourlyPricing is priceHourly's result: the owner's own rate alongside the amount the
// driver actually pays once the platform's commission is added on top (see addCommission).
type hourlyPricing struct {
	OwnerAmount int
	TotalAmount int
}

// priceHourly resolves an HOURLY booking's authoritative amount from the spot's own
// owner-set hourly rate (never trusted from input, same reasoning as priceStay) times the
// requested duration, in centavos -- same rounding convention as parking-service's own
// stay quote. addCommission then adds the platform's cut on top.
func (service *Service) priceHourly(ctx context.Context, input entity.CreateBookingInput) (hourlyPricing, error) {
	pricePerHour, err := service.parkingClient.GetHourlyPrice(ctx, input.ParkingSpotID)
	if err != nil {
		return hourlyPricing{}, err
	}
	hours := input.End.Sub(input.Start).Hours()
	ownerAmount := int(math.Round(pricePerHour * hours * 100))
	totalAmount, err := service.addCommission(ctx, ownerAmount)
	if err != nil {
		return hourlyPricing{}, err
	}
	return hourlyPricing{OwnerAmount: ownerAmount, TotalAmount: totalAmount}, nil
}

// addCommission adds the platform's current commission rate on top of ownerAmount (what
// the spot's owner is set up to receive in full) to get what the driver actually pays.
// GeneratePendingPayoutsUseCase backs the owner's share out of this same total at payout
// time, using the same rate, so NetAmount still ends up being exactly ownerAmount -- the
// owner always receives 100% of their own listed price, never the booking's gross total.
func (service *Service) addCommission(ctx context.Context, ownerAmount int) (int, error) {
	settings, err := service.commissionRates.GetCommissionRate(ctx)
	if err != nil {
		return 0, err
	}
	commission := ownerAmount * settings.CommissionRateBP / 10000
	return ownerAmount + commission, nil
}
