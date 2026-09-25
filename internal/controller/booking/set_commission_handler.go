package booking

import (
	"context"
	"net/http"

	"github.com/goccy/go-json"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/set_commission_usecase_mock.go -package=mocks -source=./set_commission_handler.go

// SetCommissionUseCase is the interface consumed by this handler for
// PUT /admin/settings/commission.
type SetCommissionUseCase interface {
	Execute(ctx context.Context, rateBP int, adminUserID string) (entity.PlatformSettings, error)
}

func (handler *Handler) SetCommissionEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	adminUserID := request.Header.Get("X-User-Id")
	if adminUserID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	// The gateway forwards the admin's role claim (SUPERADMIN/ADMIN) on every ADMIN-mode
	// route once it's verified the JWT (see gateway's app/proxy.py:_forward_request_headers)
	// -- trusted the same way X-User-Id already is. Changing the platform commission rate
	// is owner-level, not day-to-day support work, so it's restricted beyond the gateway's
	// blanket "some admin" gate, same reasoning as auth-service's admin-account management.
	if request.Header.Get("X-Admin-Role") != "SUPERADMIN" {
		writeErrorCode(responseWriter, http.StatusForbidden, "FORBIDDEN", "superadmin privileges required")
		return
	}

	var body setCommissionRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	settings, err := handler.setCommissionUseCase.Execute(request.Context(), body.CommissionRateBP, adminUserID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, commissionResponseFromEntity(settings))
}
