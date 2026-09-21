package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/admin_reply_usecase_mock.go -package=mocks -source=./admin_reply_handler.go

// AdminReplyUseCase is the interface consumed by this handler for POST /admin/tickets/{id}/messages.
type AdminReplyUseCase interface {
	Execute(ctx context.Context, ticketID, adminUserID, body string) (entity.SupportTicketMessage, error)
}

func (handler *Handler) AdminReplyEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	ticketID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ticketID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TICKET_ID", "id must be a valid UUID")
		return
	}

	// The gateway's admin auth mode still forwards X-User-Id (the JWT's sub claim) even
	// though it's the admin token, not a driver/owner one -- reused here as the reply's
	// sender_id so the thread records who on staff answered.
	adminUserID := request.Header.Get("X-User-Id")
	if adminUserID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	var body sendMessageRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	message, err := handler.adminReplyUseCase.Execute(request.Context(), ticketID, adminUserID, body.Body)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusCreated, ticketMessageResponseFromEntity(message, handler.location))
}
