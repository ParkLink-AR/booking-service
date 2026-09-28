package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_owner_pending_balances_repository_mock.go -package=mocks -source=./get_owner_pending_balances_usecase.go

// OwnerPendingBalancesGetter returns one row per owner with at least one unattached payout.
// Implemented by internal/repository/booking.
type OwnerPendingBalancesGetter interface {
	GetOwnerPendingBalances(ctx context.Context) ([]entity.OwnerPendingBalance, error)
}

// GetOwnerPendingBalancesUseCase powers GET /admin/owners/pending-balances -- the admin-panel
// screen ParkLink staff use to decide which owner to settle next.
type GetOwnerPendingBalancesUseCase struct {
	payouts OwnerPendingBalancesGetter
}

func NewGetOwnerPendingBalancesUseCase(payouts OwnerPendingBalancesGetter) *GetOwnerPendingBalancesUseCase {
	return &GetOwnerPendingBalancesUseCase{payouts: payouts}
}

func (uc *GetOwnerPendingBalancesUseCase) Execute(ctx context.Context) ([]entity.OwnerPendingBalance, error) {
	return uc.payouts.GetOwnerPendingBalances(ctx)
}
