package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_admin_payout_usecase_mock.go -package=mocks -source=./get_admin_payout_handler.go

// GetAdminPayoutUseCase is the interface consumed by this handler for GET /admin/payouts/{id}.
type GetAdminPayoutUseCase interface {
	Execute(ctx context.Context, payoutID string) (entity.Payout, error)
}

func (handler *Handler) GetAdminPayoutEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	payoutID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(payoutID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_PAYOUT_ID", "id must be a valid UUID")
		return
	}

	payout, err := handler.getAdminPayoutUseCase.Execute(request.Context(), payoutID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "PAYOUT_NOT_FOUND", "payout not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, payoutResponseFromEntity(payout, handler.location))
}
