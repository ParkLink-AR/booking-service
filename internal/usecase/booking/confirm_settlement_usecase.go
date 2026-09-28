package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/confirm_settlement_repository_mock.go -package=mocks -source=./confirm_settlement_usecase.go

// SettlementGetter fetches a single settlement by id. Implemented by
// internal/repository/booking.
type SettlementGetter interface {
	GetSettlementByID(ctx context.Context, settlementID string) (entity.Settlement, error)
}

// SettlementConfirmer flips a settlement (and every payout it covers) PENDING -> PAID,
// stamping who confirmed it and (optionally) a proof-of-transfer image URL. Returns a
// ConflictError if the settlement isn't currently PENDING (already confirmed) and a
// NotFoundError if it doesn't exist. Implemented by internal/repository/booking.
type SettlementConfirmer interface {
	ConfirmSettlement(ctx context.Context, settlementID, adminUserID string, proofURL *string) (entity.Settlement, []entity.Payout, error)
}

// SettlementStorageService uploads a settlement's bank-transfer proof image and returns its
// public URL. Implemented by internal/repository/storage (Cloudflare R2) -- reuses
// UploadPayoutProof's {ownerId}/{key}.{ext} shape, passing the settlement's own id as the key
// (payout-proofs/{ownerId}/{settlementId}.{ext}): by confirm time the settlement already has a
// real id from the earlier draft step, so there's no need for a throwaway generated filename.
type SettlementStorageService interface {
	UploadPayoutProof(ctx context.Context, ownerID, settlementID, contentType string, data []byte) (string, error)
}

// ConfirmSettlementUseCase powers POST /admin/settlements/{id}/confirm -- phase 2 of settling
// an owner's payouts, and the ONLY step that asserts the bank transfer actually went out. If a
// proof image is supplied, it's uploaded first -- a failed upload aborts the whole call, so a
// settlement is never confirmed PAID without the proof the admin thought they were attaching
// (same reasoning as the old MarkPayoutPaidUseCase). Once confirmed, it publishes
// settlement.paid to notify the owner; unlike the proof upload, a failed publish does NOT fail
// the call (see emitBookingEvent).
type ConfirmSettlementUseCase struct {
	settlementGetter SettlementGetter
	settlements      SettlementConfirmer
	storage          SettlementStorageService
	events           EventPublisher
}

func NewConfirmSettlementUseCase(
	settlementGetter SettlementGetter,
	settlements SettlementConfirmer,
	storage SettlementStorageService,
	events EventPublisher,
) *ConfirmSettlementUseCase {
	return &ConfirmSettlementUseCase{
		settlementGetter: settlementGetter, settlements: settlements, storage: storage, events: events,
	}
}

// Execute confirms settlementID on behalf of adminUserID. proof/proofContentType are the
// optional bank-transfer proof image bytes -- pass a nil/empty proof to confirm without one.
func (uc *ConfirmSettlementUseCase) Execute(
	ctx context.Context, settlementID, adminUserID string, proof []byte, proofContentType string,
) (entity.Settlement, []entity.Payout, error) {
	var proofURL *string
	if len(proof) > 0 {
		// Needed before the settlement is confirmed: the proof's storage key is
		// {ownerId}/{settlementId}, not derivable from settlementID alone.
		existing, err := uc.settlementGetter.GetSettlementByID(ctx, settlementID)
		if err != nil {
			return entity.Settlement{}, nil, err
		}
		url, err := uc.storage.UploadPayoutProof(ctx, existing.OwnerID, settlementID, proofContentType, proof)
		if err != nil {
			return entity.Settlement{}, nil, err
		}
		proofURL = &url
	}

	settlement, payouts, err := uc.settlements.ConfirmSettlement(ctx, settlementID, adminUserID, proofURL)
	if err != nil {
		return entity.Settlement{}, nil, err
	}

	emitBookingEvent(ctx, uc.events, RoutingKeySettlementPaid, newSettlementPaidEvent(settlement, len(payouts)))

	return settlement, payouts, nil
}
