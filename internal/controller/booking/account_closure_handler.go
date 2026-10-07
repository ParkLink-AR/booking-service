package booking

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/account_closure_usecase_mock.go -package=mocks -source=./account_closure_handler.go

// AccountClosureUseCase is the interface consumed by AccountClosureHandler.
type AccountClosureUseCase interface {
	Summary(ctx context.Context, userID string, spotIDs []string) (entity.AccountClosureSummary, error)
	CancelBookings(ctx context.Context, userID string, spotIDs []string) (entity.AccountClosureResult, error)
	Anonymize(ctx context.Context, userID string) error
}

// AccountClosureHandler serves the internal endpoints auth-service calls when an account
// is closed, deleted or suspended. Service-to-service only (X-Internal-Key): there's no
// gateway route for any of them.
type AccountClosureHandler struct {
	useCase            AccountClosureUseCase
	internalServiceKey string
}

func NewAccountClosureHandler(useCase AccountClosureUseCase, internalServiceKey string) *AccountClosureHandler {
	return &AccountClosureHandler{useCase: useCase, internalServiceKey: internalServiceKey}
}

func (handler *AccountClosureHandler) RegisterRoutes(router chi.Router) {
	router.Post("/internal/account-closure/summary", handler.SummaryEndpoint)
	router.Post("/internal/account-closure/cancel-bookings", handler.CancelBookingsEndpoint)
	router.Post("/internal/account-closure/anonymize", handler.AnonymizeEndpoint)
}

type accountClosureRequest struct {
	UserID  string   `json:"user_id"`
	SpotIDs []string `json:"spot_ids"`
}

type accountClosureSummaryResponse struct {
	InProgressCount        int `json:"in_progress_count"`
	DriverBookingsToCancel int `json:"driver_bookings_to_cancel"`
	OwnerBookingsToCancel  int `json:"owner_bookings_to_cancel"`
	OwnerRequestsToReject  int `json:"owner_requests_to_reject"`
	PendingPayoutCount     int `json:"pending_payout_count"`
	PendingPayoutAmount    int `json:"pending_payout_amount"`
}

type accountClosureResultResponse struct {
	Cancelled      int      `json:"cancelled"`
	ExpiredHolds   int      `json:"expired_holds"`
	RefundFailures []string `json:"refund_failures"`
}

func (handler *AccountClosureHandler) isInternalCaller(request *http.Request) bool {
	providedKey := request.Header.Get(internalKeyHeader)
	return handler.internalServiceKey != "" &&
		subtle.ConstantTimeCompare([]byte(providedKey), []byte(handler.internalServiceKey)) == 1
}

// decode reads and validates the request body, writing the error response itself and
// returning false when it isn't usable.
func (handler *AccountClosureHandler) decode(responseWriter http.ResponseWriter, request *http.Request) (accountClosureRequest, bool) {
	if !handler.isInternalCaller(request) {
		writeErrorCode(responseWriter, http.StatusUnauthorized, "INVALID_INTERNAL_KEY", "invalid internal key")
		return accountClosureRequest{}, false
	}
	var body accountClosureRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "invalid JSON body")
		return accountClosureRequest{}, false
	}
	if _, err := uuid.Parse(body.UserID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "user_id must be a valid UUID")
		return accountClosureRequest{}, false
	}
	for _, spotID := range body.SpotIDs {
		if _, err := uuid.Parse(spotID); err != nil {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "spot_ids must be valid UUIDs")
			return accountClosureRequest{}, false
		}
	}
	if body.SpotIDs == nil {
		body.SpotIDs = []string{}
	}
	return body, true
}

func (handler *AccountClosureHandler) SummaryEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	body, ok := handler.decode(responseWriter, request)
	if !ok {
		return
	}
	summary, err := handler.useCase.Summary(request.Context(), body.UserID, body.SpotIDs)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, accountClosureSummaryResponse{
		InProgressCount:        summary.InProgressCount,
		DriverBookingsToCancel: summary.DriverBookingsToCancel,
		OwnerBookingsToCancel:  summary.OwnerBookingsToCancel,
		OwnerRequestsToReject:  summary.OwnerRequestsToReject,
		PendingPayoutCount:     summary.PendingPayoutCount,
		PendingPayoutAmount:    summary.PendingPayoutAmount,
	})
}

func (handler *AccountClosureHandler) CancelBookingsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	body, ok := handler.decode(responseWriter, request)
	if !ok {
		return
	}
	result, err := handler.useCase.CancelBookings(request.Context(), body.UserID, body.SpotIDs)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	failures := result.RefundFailures
	if failures == nil {
		failures = []string{}
	}
	writeJSON(responseWriter, http.StatusOK, accountClosureResultResponse{
		Cancelled:      result.Cancelled,
		ExpiredHolds:   result.ExpiredHolds,
		RefundFailures: failures,
	})
}

func (handler *AccountClosureHandler) AnonymizeEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	body, ok := handler.decode(responseWriter, request)
	if !ok {
		return
	}
	if err := handler.useCase.Anonymize(request.Context(), body.UserID); err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]string{"status": "ok"})
}
