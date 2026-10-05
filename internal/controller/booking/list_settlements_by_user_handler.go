package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_settlements_by_user_usecase_mock.go -package=mocks -source=./list_settlements_by_user_handler.go

// ListSettlementsByOwnerUseCase is the interface consumed by this handler for
// GET /users/{id}/settlements.
type ListSettlementsByOwnerUseCase interface {
	Execute(ctx context.Context, ownerID string) ([]entity.Settlement, error)
}

// ListSettlementsByUserEndpoint is the owner's own self-service "Finanzas" history -- only
// that owner (or a ParkLink admin) may read it.
func (handler *Handler) ListSettlementsByUserEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	userID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(userID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "id must be a valid UUID")
		return
	}
	if !requireSelfOrAdmin(responseWriter, request, userID) {
		return
	}

	settlements, err := handler.listSettlementsByOwnerUseCase.Execute(request.Context(), userID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]settlementResponse, len(settlements))
	for i, settlement := range settlements {
		responses[i] = settlementResponseFromEntity(settlement, nil, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
