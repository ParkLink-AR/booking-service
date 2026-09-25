package booking

import (
	"context"
	"net/http"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_commission_usecase_mock.go -package=mocks -source=./get_commission_handler.go

// GetCommissionUseCase is the interface consumed by this handler for
// GET /admin/settings/commission.
type GetCommissionUseCase interface {
	Execute(ctx context.Context) (entity.PlatformSettings, error)
}

func (handler *Handler) GetCommissionEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	settings, err := handler.getCommissionUseCase.Execute(request.Context())
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, commissionResponseFromEntity(settings))
}
