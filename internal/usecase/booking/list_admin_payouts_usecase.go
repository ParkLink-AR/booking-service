package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_payouts_repository_mock.go -package=mocks -source=./list_admin_payouts_usecase.go

// PayoutLister returns every payout, optionally filtered by status. Implemented by
// internal/repository/booking.
type PayoutLister interface {
	ListPayouts(ctx context.Context, status *enum.PayoutStatus) ([]entity.Payout, error)
}

// ListAdminPayoutsUseCase powers GET /admin/payouts. There's no per-caller ownership check
// here -- the gateway only lets this route through for a caller whose JWT carries
// role:"admin", so every payout is fair game.
type ListAdminPayoutsUseCase struct {
	payouts PayoutLister
}

func NewListAdminPayoutsUseCase(payouts PayoutLister) *ListAdminPayoutsUseCase {
	return &ListAdminPayoutsUseCase{payouts: payouts}
}

func (uc *ListAdminPayoutsUseCase) Execute(ctx context.Context, status *enum.PayoutStatus) ([]entity.Payout, error) {
	return uc.payouts.ListPayouts(ctx, status)
}
