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
	Execute(ctx context.Context, status *enum.PayoutStatus, ownerID *string) ([]entity.Payout, error)
}

// parseOptionalOwnerID returns nil when raw is empty (the query param was omitted) -- unlike
// path params elsewhere in this package, this one isn't UUID-validated: an unrecognized owner
// id simply yields an empty result set, no different from a typo'd status.
func parseOptionalOwnerID(raw string) *string {
	if raw == "" {
		return nil
	}
	return &raw
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

	ownerID := parseOptionalOwnerID(request.URL.Query().Get("owner_id"))

	payouts, err := handler.listAdminPayoutsUseCase.Execute(request.Context(), status, ownerID)
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
