package booking

import "context"

//go:generate go tool mockgen -destination=../../../pkg/mocks/cancel_settlement_repository_mock.go -package=mocks -source=./cancel_settlement_usecase.go

// SettlementCanceller undoes a settlement draft before it's confirmed, releasing every payout
// it covered back to unattached. Returns a ConflictError if the settlement has already been
// confirmed (PAID) and a NotFoundError if it doesn't exist. Implemented by
// internal/repository/booking.
type SettlementCanceller interface {
	CancelSettlement(ctx context.Context, settlementID string) error
}

// CancelSettlementUseCase powers POST /admin/settlements/{id}/cancel -- lets an admin undo a
// mistaken draft. It can never be used to undo a confirmed (PAID) settlement, since that
// represents money that has actually moved.
type CancelSettlementUseCase struct {
	settlements SettlementCanceller
}

func NewCancelSettlementUseCase(settlements SettlementCanceller) *CancelSettlementUseCase {
	return &CancelSettlementUseCase{settlements: settlements}
}

func (uc *CancelSettlementUseCase) Execute(ctx context.Context, settlementID string) error {
	return uc.settlements.CancelSettlement(ctx, settlementID)
}
