package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_ticket_messages_usecase_mock.go -package=mocks -source=./list_admin_ticket_messages_handler.go

// ListAdminTicketMessagesUseCase is the interface consumed by this handler for GET /admin/tickets/{id}/messages.
type ListAdminTicketMessagesUseCase interface {
	Execute(ctx context.Context, ticketID string) ([]entity.SupportTicketMessage, error)
}

func (handler *Handler) ListAdminTicketMessagesEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	ticketID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ticketID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TICKET_ID", "id must be a valid UUID")
		return
	}

	messages, err := handler.listAdminTicketMessagesUseCase.Execute(request.Context(), ticketID)
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
