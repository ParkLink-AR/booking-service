package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/cancel_settlement_usecase_mock.go -package=mocks -source=./cancel_settlement_handler.go

// CancelSettlementUseCase is the interface consumed by this handler for
// POST /admin/settlements/{id}/cancel.
type CancelSettlementUseCase interface {
	Execute(ctx context.Context, settlementID string) error
}

// CancelSettlementEndpoint lets an admin undo a mistaken draft before it's confirmed,
// releasing every payout it covered back to unattached.
func (handler *Handler) CancelSettlementEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	settlementID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(settlementID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SETTLEMENT_ID", "id must be a valid UUID")
		return
	}

	if err := handler.cancelSettlementUseCase.Execute(request.Context(), settlementID); err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "SETTLEMENT_NOT_FOUND", "settlement not found")
			return
		}
		if errors.IsConflict(err) {
			writeErrorCode(responseWriter, http.StatusConflict, "ALREADY_CONFIRMED", err.Error())
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}
