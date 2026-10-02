package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_own_settlement_repository_mock.go -package=mocks -source=./get_own_settlement_usecase.go

// GetOwnSettlementUseCase powers GET /settlements/{id} -- the owner's own self-service detail
// view. Unlike GetSettlementUseCase (admin), this is reachable by any logged-in user's token,
// so it enforces that the caller is the settlement's own owner -- a ForbiddenError, same
// convention as this codebase's other "not yours" checks (see e.g. GetTicketUseCase). A still-
// PENDING draft is visible here too (see ListSettlementsByOwnerUseCase) -- the owner sees it as
// "being prepared" until ConfirmSettlementUseCase flips it to PAID.
type GetOwnSettlementUseCase struct {
	settlements SettlementGetter
	payouts     PayoutsBySettlementLister
}

func NewGetOwnSettlementUseCase(settlements SettlementGetter, payouts PayoutsBySettlementLister) *GetOwnSettlementUseCase {
	return &GetOwnSettlementUseCase{settlements: settlements, payouts: payouts}
}

func (uc *GetOwnSettlementUseCase) Execute(ctx context.Context, settlementID, callerID string) (entity.Settlement, []entity.Payout, error) {
	settlement, err := uc.settlements.GetSettlementByID(ctx, settlementID)
	if err != nil {
		return entity.Settlement{}, nil, err
	}
	if callerID != settlement.OwnerID {
		return entity.Settlement{}, nil, errors.NewForbidden("no sos el dueño de esta liquidación")
	}
	payouts, err := uc.payouts.ListPayoutsBySettlement(ctx, settlementID)
	if err != nil {
		return entity.Settlement{}, nil, err
	}
	return settlement, payouts, nil
}
