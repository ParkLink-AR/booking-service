package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_settlements_repository_mock.go -package=mocks -source=./list_admin_settlements_usecase.go

// SettlementLister returns every settlement, optionally filtered by status. Implemented by
// internal/repository/booking.
type SettlementLister interface {
	ListSettlements(ctx context.Context, status *enum.SettlementStatus) ([]entity.Settlement, error)
}

// ListAdminSettlementsUseCase powers GET /admin/settlements -- lets admin-backend show drafts
// awaiting confirmation ("Liquidaciones a confirmar") separately from confirmed history
// ("Liquidaciones pagadas"). There's no per-caller ownership check here -- gateway-gated to
// admins only, same as ListAdminPayoutsUseCase.
type ListAdminSettlementsUseCase struct {
	settlements SettlementLister
	proofURLs   ProofURLResolver
}

func NewListAdminSettlementsUseCase(settlements SettlementLister, proofURLs ProofURLResolver) *ListAdminSettlementsUseCase {
	return &ListAdminSettlementsUseCase{settlements: settlements, proofURLs: proofURLs}
}

func (uc *ListAdminSettlementsUseCase) Execute(ctx context.Context, status *enum.SettlementStatus) ([]entity.Settlement, error) {
	settlements, err := uc.settlements.ListSettlements(ctx, status)
	if err != nil {
		return nil, err
	}
	for i := range settlements {
		if err := resolveSettlementProofURL(ctx, uc.proofURLs, &settlements[i]); err != nil {
			return nil, err
		}
	}
	return settlements, nil
}
