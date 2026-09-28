package booking

import (
	"context"
	"net/http"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_owner_pending_balances_usecase_mock.go -package=mocks -source=./get_owner_pending_balances_handler.go

// GetOwnerPendingBalancesUseCase is the interface consumed by this handler for
// GET /admin/owners/pending-balances.
type GetOwnerPendingBalancesUseCase interface {
	Execute(ctx context.Context) ([]entity.OwnerPendingBalance, error)
}

func (handler *Handler) GetOwnerPendingBalancesEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	balances, err := handler.getOwnerPendingBalancesUseCase.Execute(request.Context())
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]ownerPendingBalanceResponse, len(balances))
	for i, balance := range balances {
		responses[i] = ownerPendingBalanceResponseFromEntity(balance, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
