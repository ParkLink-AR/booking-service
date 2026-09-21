package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_ticket_messages_usecase_mock.go -package=mocks -source=./list_ticket_messages_handler.go

// ListTicketMessagesUseCase is the interface consumed by this handler for GET /tickets/{id}/messages.
type ListTicketMessagesUseCase interface {
	Execute(ctx context.Context, ticketID, callerID string) ([]entity.SupportTicketMessage, error)
}

func (handler *Handler) ListTicketMessagesEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	ticketID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ticketID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TICKET_ID", "id must be a valid UUID")
		return
	}

	callerID := request.Header.Get("X-User-Id")
	if callerID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	messages, err := handler.listTicketMessagesUseCase.Execute(request.Context(), ticketID, callerID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]ticketMessageResponse, len(messages))
	for i, message := range messages {
		responses[i] = ticketMessageResponseFromEntity(message, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
