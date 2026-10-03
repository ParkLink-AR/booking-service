package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_settlement_repository_mock.go -package=mocks -source=./get_settlement_usecase.go

// PayoutsBySettlementLister returns every payout a settlement covers. Implemented by
// internal/repository/booking.
type PayoutsBySettlementLister interface {
	ListPayoutsBySettlement(ctx context.Context, settlementID string) ([]entity.Payout, error)
}

// GetSettlementUseCase powers GET /admin/settlements/{id}. There's no per-caller ownership
// check here -- the gateway only lets this route through for a caller whose JWT carries
// role:"admin", so any settlement, drafted or confirmed, is fair game.
type GetSettlementUseCase struct {
	settlements SettlementGetter
	payouts     PayoutsBySettlementLister
	proofURLs   ProofURLResolver
}

func NewGetSettlementUseCase(settlements SettlementGetter, payouts PayoutsBySettlementLister, proofURLs ProofURLResolver) *GetSettlementUseCase {
	return &GetSettlementUseCase{settlements: settlements, payouts: payouts, proofURLs: proofURLs}
}

func (uc *GetSettlementUseCase) Execute(ctx context.Context, settlementID string) (entity.Settlement, []entity.Payout, error) {
	settlement, err := uc.settlements.GetSettlementByID(ctx, settlementID)
	if err != nil {
		return entity.Settlement{}, nil, err
	}
	payouts, err := uc.payouts.ListPayoutsBySettlement(ctx, settlementID)
	if err != nil {
		return entity.Settlement{}, nil, err
	}
	if err := resolveSettlementProofURL(ctx, uc.proofURLs, &settlement); err != nil {
		return entity.Settlement{}, nil, err
	}
	return settlement, payouts, nil
}
