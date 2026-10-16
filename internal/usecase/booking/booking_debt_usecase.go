package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_debt_repository_mock.go -package=mocks -source=./booking_debt_usecase.go

// BookingDebtRepository reads what drivers owe for staying past the end of a booking. Implemented
// by internal/repository/booking.
type BookingDebtRepository interface {
	ListDebtsByUser(ctx context.Context, userID string) ([]entity.BookingDebt, error)
	ListDebtsChargedTo(ctx context.Context, bookingID string) ([]entity.BookingDebt, error)
	ListDebtsOfBooking(ctx context.Context, bookingID string) ([]entity.BookingDebt, error)
}

// BookingDebtUseCase tells a driver what they owe before they commit to a new reservation, what a
// booking's payment includes (payment-service words the checkout with it) and, for support, which
// debts a booking is part of. Nothing here charges anything: a debt rides on the driver's next
// booking when it is created (see Repository.CreateBooking) and is settled when that booking's
// payment is approved.
type BookingDebtUseCase struct {
	debts BookingDebtRepository
	spots SpotDetailsLookup
}

func NewBookingDebtUseCase(debts BookingDebtRepository, spots SpotDetailsLookup) *BookingDebtUseCase {
	return &BookingDebtUseCase{debts: debts, spots: spots}
}

// Mine is userID's debts, newest first, and how much a new booking would add to its price right
// now (the pending ones; a reserved one already rides on a booking waiting for its payment).
func (uc *BookingDebtUseCase) Mine(ctx context.Context, userID string) (entity.DebtSummary, error) {
	debts, err := uc.debts.ListDebtsByUser(ctx, userID)
	if err != nil {
		return entity.DebtSummary{}, err
	}
	summary := entity.DebtSummary{Debts: uc.named(ctx, debts)}
	for _, debt := range debts {
		if debt.Status == entity.DebtPending {
			summary.PendingAmount += debt.Amount
		}
	}
	return summary, nil
}

// ChargedTo is the debts bookingID's payment includes.
func (uc *BookingDebtUseCase) ChargedTo(ctx context.Context, bookingID string) ([]entity.BookingDebt, error) {
	debts, err := uc.debts.ListDebtsChargedTo(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	return uc.named(ctx, debts), nil
}

// OfBooking is every debt bookingID is part of: the one it caused by being overstayed and the
// ones its payment included.
func (uc *BookingDebtUseCase) OfBooking(ctx context.Context, bookingID string) ([]entity.BookingDebt, error) {
	debts, err := uc.debts.ListDebtsOfBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	return uc.named(ctx, debts), nil
}

// named fills in the title of the spot each debt was caused at. A spot parking-service can't
// resolve right now only leaves its debts untitled: what the driver owes doesn't depend on it.
func (uc *BookingDebtUseCase) named(ctx context.Context, debts []entity.BookingDebt) []entity.BookingDebt {
	titles := make(map[string]string)
	for index := range debts {
		spotID := debts[index].ParkingSpotID
		title, known := titles[spotID]
		if !known {
			if spot, err := uc.spots.GetSpotDetails(ctx, spotID); err == nil {
				title = spot.Title
			}
			titles[spotID] = title
		}
		debts[index].SpotTitle = title
	}
	return debts
}
