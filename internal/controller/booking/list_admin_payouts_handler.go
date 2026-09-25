package booking

import (
	"context"
	"net/http"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_payouts_usecase_mock.go -package=mocks -source=./list_admin_payouts_handler.go

// ListAdminPayoutsUseCase is the interface consumed by this handler for GET /admin/payouts.
type ListAdminPayoutsUseCase interface {
	Execute(ctx context.Context, status *enum.PayoutStatus) ([]entity.Payout, error)
}

// parseOptionalPayoutStatus maps the ?status= query param ("pending"/"paid", lowercase to
// match the admin panel's own convention for this endpoint) to the domain enum. Returns
// (nil, true) when raw is empty.
func parseOptionalPayoutStatus(raw string) (*enum.PayoutStatus, bool) {
	switch raw {
	case "":
		return nil, true
	case "pending":
		status := enum.PayoutStatusPending
		return &status, true
	case "paid":
		status := enum.PayoutStatusPaid
		return &status, true
	default:
		return nil, false
	}
}

func (handler *Handler) ListAdminPayoutsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	status, ok := parseOptionalPayoutStatus(request.URL.Query().Get("status"))
	if !ok {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_STATUS", "status must be one of pending, paid")
		return
	}

	payouts, err := handler.listAdminPayoutsUseCase.Execute(request.Context(), status)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]payoutResponse, len(payouts))
	for i, payout := range payouts {
		responses[i] = payoutResponseFromEntity(payout, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
