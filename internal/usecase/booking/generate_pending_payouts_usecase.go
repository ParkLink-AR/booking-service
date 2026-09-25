package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/generate_pending_payouts_repository_mock.go -package=mocks -source=./generate_pending_payouts_usecase.go

// EligibleBookingLister returns COMPLETED, actually-paid bookings whose 24h payout
// eligibility window has elapsed and that don't already have a payout row. Implemented by
// internal/repository/booking.
type EligibleBookingLister interface {
	ListEligibleBookingsWithoutPayout(ctx context.Context) ([]entity.Booking, error)
}

// PayoutCreator inserts a new PENDING payout row. Implemented by internal/repository/booking.
type PayoutCreator interface {
	CreatePayout(ctx context.Context, payout entity.Payout) (entity.Payout, error)
}

// CommissionRateGetter reads the platform's current commission settings. Implemented by
// internal/repository/booking.
type CommissionRateGetter interface {
	GetCommissionRate(ctx context.Context) (entity.PlatformSettings, error)
}

// GeneratePendingPayoutsUseCase turns every newly-eligible booking into a PENDING payout
// row, at the platform commission rate in effect at generation time. Meant to run on a
// ticker (see cmd.runPayoutEligibilityJob) -- a single booking failing to resolve its spot
// or insert its payout is skipped rather than aborting the whole batch: the rest are
// independent, and a skipped booking simply remains eligible (still no payout row) and gets
// picked up again on the next tick, matching this codebase's existing sweep-job philosophy
// that a partial failure shouldn't block the whole batch (see ExpireStalePendingApprovals).
type GeneratePendingPayoutsUseCase struct {
	bookings EligibleBookingLister
	payouts  PayoutCreator
	settings CommissionRateGetter
	spots    SpotDetailsLookup
}

func NewGeneratePendingPayoutsUseCase(
	bookings EligibleBookingLister,
	payouts PayoutCreator,
	settings CommissionRateGetter,
	spots SpotDetailsLookup,
) *GeneratePendingPayoutsUseCase {
	return &GeneratePendingPayoutsUseCase{bookings: bookings, payouts: payouts, settings: settings, spots: spots}
}

// Execute returns how many payouts it created, so the caller can log it. A single booking's
// spot lookup or payout insert failing is swallowed and simply skipped -- see the type doc.
func (uc *GeneratePendingPayoutsUseCase) Execute(ctx context.Context) (int, error) {
	bookings, err := uc.bookings.ListEligibleBookingsWithoutPayout(ctx)
	if err != nil {
		return 0, err
	}
	if len(bookings) == 0 {
		return 0, nil
	}

	settings, err := uc.settings.GetCommissionRate(ctx)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, b := range bookings {
		spot, err := uc.spots.GetSpotDetails(ctx, b.ParkingSpotID)
		if err != nil {
			continue
		}

		commission := b.Amount * settings.CommissionRateBP / 10000
		_, err = uc.payouts.CreatePayout(ctx, entity.Payout{
			BookingID:        b.ID,
			OwnerID:          spot.OwnerID,
			PaymentID:        b.PaymentID,
			GrossAmount:      b.Amount,
			CommissionAmount: commission,
			NetAmount:        b.Amount - commission,
		})
		if err != nil {
			continue
		}
		created++
	}
	return created, nil
}
