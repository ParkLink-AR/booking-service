package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_admin_payout_repository_mock.go -package=mocks -source=./get_admin_payout_usecase.go

// PayoutGetter fetches a single payout by id. Implemented by internal/repository/booking.
type PayoutGetter interface {
	GetPayoutByID(ctx context.Context, payoutID string) (entity.Payout, error)
}

// GetAdminPayoutUseCase powers GET /admin/payouts/{id}.
type GetAdminPayoutUseCase struct {
	payouts PayoutGetter
}

func NewGetAdminPayoutUseCase(payouts PayoutGetter) *GetAdminPayoutUseCase {
	return &GetAdminPayoutUseCase{payouts: payouts}
}

func (uc *GetAdminPayoutUseCase) Execute(ctx context.Context, payoutID string) (entity.Payout, error) {
	return uc.payouts.GetPayoutByID(ctx, payoutID)
}
