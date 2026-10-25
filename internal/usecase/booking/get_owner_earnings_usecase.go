package booking

import (
	"context"
	"sort"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

// ownerEarningsMonths is how many calendar months GetOwnerEarningsUseCase breaks down.
const ownerEarningsMonths = 6

// GetOwnerEarningsUseCase powers GET /users/{id}/earnings: an owner's real earnings from
// their own bookings and settlements, for the app's "Mis finanzas" screen.
type GetOwnerEarningsUseCase struct {
	spots       OwnerSpotIDsLookup
	bookings    OwnerSpotBookingLister
	settlements OwnerSettlementLister
	settings    CommissionRateGetter
	location    *time.Location
	now         func() time.Time
}

func NewGetOwnerEarningsUseCase(
	spots OwnerSpotIDsLookup,
	bookings OwnerSpotBookingLister,
	settlements OwnerSettlementLister,
	settings CommissionRateGetter,
	location *time.Location,
	now func() time.Time,
) *GetOwnerEarningsUseCase {
	return &GetOwnerEarningsUseCase{
		spots: spots, bookings: bookings, settlements: settlements, settings: settings, location: location, now: now,
	}
}

func (uc *GetOwnerEarningsUseCase) Execute(ctx context.Context, ownerID string) (entity.OwnerEarnings, error) {
	spotIDs, err := uc.spots.GetSpotIDsForOwner(ctx, ownerID)
	if err != nil {
		return entity.OwnerEarnings{}, err
	}
	var bookings []entity.Booking
	if len(spotIDs) > 0 {
		if bookings, err = uc.bookings.ListBookingsByOwnerSpots(ctx, spotIDs); err != nil {
			return entity.OwnerEarnings{}, err
		}
	}
	paid := enum.SettlementStatusPaid
	settlements, err := uc.settlements.ListSettlementsByOwner(ctx, ownerID, &paid)
	if err != nil {
		return entity.OwnerEarnings{}, err
	}
	settings, err := uc.settings.GetCommissionRate(ctx)
	if err != nil {
		return entity.OwnerEarnings{}, err
	}
	return summarizeOwnerEarnings(bookings, settlements, settings.CommissionRateBP, uc.now().In(uc.location)), nil
}

// ownerShare is the owner's part of a booking once any refund went back to the driver --
// the same split GeneratePendingPayoutsUseCase pays out.
func ownerShare(b entity.Booking, commissionRateBP int) int {
	var commission int
	if b.OwnerAmount != nil {
		commission = b.Amount - *b.OwnerAmount
	} else {
		commission = b.Amount * commissionRateBP / (10000 + commissionRateBP)
	}
	_, _, net := refundedPayoutAmounts(b.Amount, commission, b.RefundedAmount)
	return net
}

// earnsOwner reports whether a booking pays its owner: finished, or cancelled by the driver
// in its last hour (the owner keeps what wasn't refunded) -- and never one ParkLink refunded
// because the arrival couldn't be confirmed. Same pool as ListEligibleBookingsWithoutPayout.
func earnsOwner(b entity.Booking) bool {
	if b.PaidAt == nil {
		return false
	}
	if b.VerificationReview != nil && *b.VerificationReview == enum.VerificationReviewRefunded {
		return false
	}
	lateCancel := b.Status == enum.BookingStatusCancelled &&
		b.CancellationReason != nil && *b.CancellationReason == constants.CancellationReasonDriverLateCancelled
	return b.Status == enum.BookingStatusCompleted || lateCancel
}

func summarizeOwnerEarnings(
	bookings []entity.Booking, paidSettlements []entity.Settlement, commissionRateBP int, now time.Time,
) entity.OwnerEarnings {
	location := now.Location()
	firstMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location).AddDate(0, -(ownerEarningsMonths - 1), 0)
	months := make([]entity.MonthEarnings, ownerEarningsMonths)
	monthIndex := make(map[string]int, ownerEarningsMonths)
	for i := range months {
		label := firstMonth.AddDate(0, i, 0).Format("2006-01")
		months[i] = entity.MonthEarnings{Month: label}
		monthIndex[label] = i
	}

	summary := entity.OwnerEarnings{}
	bySpot := map[string]*entity.SpotEarnings{}
	for _, b := range bookings {
		switch {
		case earnsOwner(b):
			share := ownerShare(b, commissionRateBP)
			summary.TotalEarned += share
			index, recent := monthIndex[b.End.In(location).Format("2006-01")]
			if !recent {
				continue
			}
			months[index].Earned += share
			months[index].Bookings++
			spot := bySpot[b.ParkingSpotID]
			if spot == nil {
				spot = &entity.SpotEarnings{ParkingSpotID: b.ParkingSpotID}
				bySpot[b.ParkingSpotID] = spot
			}
			spot.Earned += share
			spot.Bookings++
		case b.PaidAt != nil && (b.Status == enum.BookingStatusConfirmed || b.Status == enum.BookingStatusInProgress):
			summary.Upcoming += ownerShare(b, commissionRateBP)
			summary.UpcomingCount++
		}
	}

	for _, settlement := range paidSettlements {
		summary.PaidOut += settlement.NetAmount
	}
	summary.ToCollect = max(0, summary.TotalEarned-summary.PaidOut)
	summary.Months = months
	summary.Spots = make([]entity.SpotEarnings, 0, len(bySpot))
	for _, spot := range bySpot {
		summary.Spots = append(summary.Spots, *spot)
	}
	sort.Slice(summary.Spots, func(i, j int) bool {
		if summary.Spots[i].Earned != summary.Spots[j].Earned {
			return summary.Spots[i].Earned > summary.Spots[j].Earned
		}
		return summary.Spots[i].ParkingSpotID < summary.Spots[j].ParkingSpotID
	})
	return summary
}
