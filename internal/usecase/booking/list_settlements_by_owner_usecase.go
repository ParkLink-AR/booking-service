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
// self-service "Finanzas" history. Always filters to PAID: a drafted-but-unconfirmed
// settlement isn't "their money released" yet, so it has no reason to appear in an owner's
// own list, same reasoning as GetOwnSettlementUseCase's detail view.
type ListSettlementsByOwnerUseCase struct {
	settlements OwnerSettlementLister
}

func NewListSettlementsByOwnerUseCase(settlements OwnerSettlementLister) *ListSettlementsByOwnerUseCase {
	return &ListSettlementsByOwnerUseCase{settlements: settlements}
}

func (uc *ListSettlementsByOwnerUseCase) Execute(ctx context.Context, ownerID string) ([]entity.Settlement, error) {
	paid := enum.SettlementStatusPaid
	return uc.settlements.ListSettlementsByOwner(ctx, ownerID, &paid)
}
