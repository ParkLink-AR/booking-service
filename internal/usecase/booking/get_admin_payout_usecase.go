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
	payouts   PayoutGetter
	proofURLs ProofURLResolver
}

func NewGetAdminPayoutUseCase(payouts PayoutGetter, proofURLs ProofURLResolver) *GetAdminPayoutUseCase {
	return &GetAdminPayoutUseCase{payouts: payouts, proofURLs: proofURLs}
}

func (uc *GetAdminPayoutUseCase) Execute(ctx context.Context, payoutID string) (entity.Payout, error) {
	payout, err := uc.payouts.GetPayoutByID(ctx, payoutID)
	if err != nil {
		return entity.Payout{}, err
	}
	if err := resolvePayoutProofURL(ctx, uc.proofURLs, &payout); err != nil {
		return entity.Payout{}, err
	}
	return payout, nil
}
