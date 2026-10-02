package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_settlements_by_owner_repository_mock.go -package=mocks -source=./list_settlements_by_owner_usecase.go

// OwnerSettlementLister returns an owner's settlements, optionally filtered by status.
// Implemented by internal/repository/booking.
type OwnerSettlementLister interface {
	ListSettlementsByOwner(ctx context.Context, ownerID string, status *enum.SettlementStatus) ([]entity.Settlement, error)
}

// ListSettlementsByOwnerUseCase powers GET /users/{id}/settlements -- the owner's own
// self-service "Finanzas" history. Returns both PENDING and PAID settlements, unfiltered: an
// admin drafting a settlement (DraftSettlementUseCase) is a visible, owner-facing event --
// the owner should see "we're preparing your payout" as soon as it happens, not only once it's
// confirmed PAID. Same reasoning applies to GetOwnSettlementUseCase's detail view.
type ListSettlementsByOwnerUseCase struct {
	settlements OwnerSettlementLister
}

func NewListSettlementsByOwnerUseCase(settlements OwnerSettlementLister) *ListSettlementsByOwnerUseCase {
	return &ListSettlementsByOwnerUseCase{settlements: settlements}
}

func (uc *ListSettlementsByOwnerUseCase) Execute(ctx context.Context, ownerID string) ([]entity.Settlement, error) {
	return uc.settlements.ListSettlementsByOwner(ctx, ownerID, nil)
}
