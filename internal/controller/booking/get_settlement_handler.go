package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_own_settlement_usecase_mock.go -package=mocks -source=./get_settlement_handler.go

// GetOwnSettlementUseCase is the interface consumed by this handler for GET /settlements/{id}.
type GetOwnSettlementUseCase interface {
	Execute(ctx context.Context, settlementID, callerID string) (entity.Settlement, []entity.Payout, error)
}

// GetSettlementEndpoint is the owner's own self-service settlement detail view. Unlike
// GetAdminSettlementEndpoint, this is reachable by any logged-in user's token (not
// gateway-ADMIN-gated), so it MUST check the caller owns the settlement -- enforced by
// GetOwnSettlementUseCase, which also hides a still-PENDING draft from its own owner.
func (handler *Handler) GetSettlementEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	settlementID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(settlementID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SETTLEMENT_ID", "id must be a valid UUID")
		return
	}

	callerID := request.Header.Get("X-User-Id")
	if callerID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	settlement, payouts, err := handler.getOwnSettlementUseCase.Execute(request.Context(), settlementID, callerID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "SETTLEMENT_NOT_FOUND", "settlement not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, settlementResponseFromEntity(settlement, payouts, handler.location))
}
