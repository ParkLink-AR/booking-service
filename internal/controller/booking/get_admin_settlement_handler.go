package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_admin_settlement_usecase_mock.go -package=mocks -source=./get_admin_settlement_handler.go

// GetSettlementUseCase is the interface consumed by this handler for
// GET /admin/settlements/{id}.
type GetSettlementUseCase interface {
	Execute(ctx context.Context, settlementID string) (entity.Settlement, []entity.Payout, error)
}

func (handler *Handler) GetAdminSettlementEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	settlementID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(settlementID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SETTLEMENT_ID", "id must be a valid UUID")
		return
	}

	settlement, payouts, err := handler.getSettlementUseCase.Execute(request.Context(), settlementID)
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
