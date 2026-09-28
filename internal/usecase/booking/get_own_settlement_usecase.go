package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_own_settlement_repository_mock.go -package=mocks -source=./get_own_settlement_usecase.go

// GetOwnSettlementUseCase powers GET /settlements/{id} -- the owner's own self-service detail
// view. Unlike GetSettlementUseCase (admin), this is reachable by any logged-in user's token,
// so it enforces that the caller is the settlement's own owner -- a ForbiddenError, same
// convention as this codebase's other "not yours" checks (see e.g. GetTicketUseCase). It also
// never reveals a still-PENDING draft to its own owner: an owner has no reason to see a
// settlement an admin hasn't confirmed yet, since it isn't "their money released" until
// confirmation -- surfaced as NotFoundError, since as far as the owner's own view is
// concerned nothing settleable exists there yet.
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
	if settlement.Status != enum.SettlementStatusPaid {
		return entity.Settlement{}, nil, errors.NewNotFound("settlement", settlementID)
	}
	payouts, err := uc.payouts.ListPayoutsBySettlement(ctx, settlementID)
	if err != nil {
		return entity.Settlement{}, nil, err
	}
	return settlement, payouts, nil
}
