package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/draft_settlement_usecase_mock.go -package=mocks -source=./draft_settlement_handler.go

// DraftSettlementUseCase is the interface consumed by this handler for
// POST /admin/owners/{id}/settlements.
type DraftSettlementUseCase interface {
	Execute(ctx context.Context, ownerID, adminUserID string) (entity.Settlement, []entity.Payout, error)
}

// DraftSettlementEndpoint is phase 1 of settling an owner's payouts (see entity.Settlement):
// it only freezes the amount currently owed into a new PENDING settlement -- no multipart body,
// no proof, no money-moved assertion. That's ConfirmSettlementEndpoint's job.
func (handler *Handler) DraftSettlementEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	ownerID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ownerID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_OWNER_ID", "id must be a valid UUID")
		return
	}

	adminUserID := request.Header.Get("X-User-Id")
	if adminUserID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	settlement, payouts, err := handler.draftSettlementUseCase.Execute(request.Context(), ownerID, adminUserID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusCreated, settlementResponseFromEntity(settlement, payouts, handler.location))
}
