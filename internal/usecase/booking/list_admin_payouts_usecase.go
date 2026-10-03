package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_payouts_repository_mock.go -package=mocks -source=./list_admin_payouts_usecase.go

// PayoutLister returns every payout, optionally filtered by status and/or owner id.
// Implemented by internal/repository/booking.
type PayoutLister interface {
	ListPayouts(ctx context.Context, status *enum.PayoutStatus, ownerID *string) ([]entity.Payout, error)
}

// ListAdminPayoutsUseCase powers GET /admin/payouts. There's no per-caller ownership check
// here -- the gateway only lets this route through for a caller whose JWT carries
// role:"admin", so every payout is fair game.
type ListAdminPayoutsUseCase struct {
	payouts   PayoutLister
	proofURLs ProofURLResolver
}

func NewListAdminPayoutsUseCase(payouts PayoutLister, proofURLs ProofURLResolver) *ListAdminPayoutsUseCase {
	return &ListAdminPayoutsUseCase{payouts: payouts, proofURLs: proofURLs}
}

func (uc *ListAdminPayoutsUseCase) Execute(ctx context.Context, status *enum.PayoutStatus, ownerID *string) ([]entity.Payout, error) {
	payouts, err := uc.payouts.ListPayouts(ctx, status, ownerID)
	if err != nil {
		return nil, err
	}
	for i := range payouts {
		if err := resolvePayoutProofURL(ctx, uc.proofURLs, &payouts[i]); err != nil {
			return nil, err
		}
	}
	return payouts, nil
}
