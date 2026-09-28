package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/draft_settlement_repository_mock.go -package=mocks -source=./draft_settlement_usecase.go

// SettlementDrafter atomically sweeps up every unattached payout for ownerID into a new
// PENDING settlement -- see entity.Settlement for the two-phase draft/confirm rationale.
// Returns a NOTHING_TO_SETTLE InvalidInputError if the owner has no unattached payouts.
// Implemented by internal/repository/booking.
type SettlementDrafter interface {
	CreateSettlementDraft(ctx context.Context, ownerID, adminUserID string) (entity.Settlement, []entity.Payout, error)
}

// DraftSettlementUseCase powers POST /admin/owners/{id}/settlements -- phase 1 of settling an
// owner's payouts. It only freezes the amount owed; it does NOT assert any money has moved
// and does NOT publish any event -- that's ConfirmSettlementUseCase's job, once an admin
// confirms the bank transfer actually went out.
type DraftSettlementUseCase struct {
	settlements SettlementDrafter
}

func NewDraftSettlementUseCase(settlements SettlementDrafter) *DraftSettlementUseCase {
	return &DraftSettlementUseCase{settlements: settlements}
}

func (uc *DraftSettlementUseCase) Execute(ctx context.Context, ownerID, adminUserID string) (entity.Settlement, []entity.Payout, error) {
	return uc.settlements.CreateSettlementDraft(ctx, ownerID, adminUserID)
}
