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
	proofURLs   ProofURLResolver
}

func NewListSettlementsByOwnerUseCase(settlements OwnerSettlementLister, proofURLs ProofURLResolver) *ListSettlementsByOwnerUseCase {
	return &ListSettlementsByOwnerUseCase{settlements: settlements, proofURLs: proofURLs}
}

func (uc *ListSettlementsByOwnerUseCase) Execute(ctx context.Context, ownerID string) ([]entity.Settlement, error) {
	settlements, err := uc.settlements.ListSettlementsByOwner(ctx, ownerID, nil)
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
