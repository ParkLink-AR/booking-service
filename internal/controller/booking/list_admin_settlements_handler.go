package booking

import (
	"context"
	"net/http"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_settlements_usecase_mock.go -package=mocks -source=./list_admin_settlements_handler.go

// ListAdminSettlementsUseCase is the interface consumed by this handler for
// GET /admin/settlements.
type ListAdminSettlementsUseCase interface {
	Execute(ctx context.Context, status *enum.SettlementStatus) ([]entity.Settlement, error)
}

// parseOptionalSettlementStatus maps the ?status= query param ("pending"/"paid", same
// lowercase convention as ListAdminPayoutsEndpoint) to the domain enum. Returns (nil, true)
// when raw is empty.
func parseOptionalSettlementStatus(raw string) (*enum.SettlementStatus, bool) {
	switch raw {
	case "":
		return nil, true
	case "pending":
		status := enum.SettlementStatusPending
		return &status, true
	case "paid":
		status := enum.SettlementStatusPaid
		return &status, true
	default:
		return nil, false
	}
}

func (handler *Handler) ListAdminSettlementsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	status, ok := parseOptionalSettlementStatus(request.URL.Query().Get("status"))
	if !ok {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_STATUS", "status must be one of pending, paid")
		return
	}

	settlements, err := handler.listAdminSettlementsUseCase.Execute(request.Context(), status)
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
