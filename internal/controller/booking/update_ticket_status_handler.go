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

//go:generate go tool mockgen -destination=../../../pkg/mocks/update_ticket_status_usecase_mock.go -package=mocks -source=./update_ticket_status_handler.go

// UpdateTicketStatusUseCase is the interface consumed by this handler for PATCH /admin/tickets/{id}/status.
type UpdateTicketStatusUseCase interface {
	Execute(ctx context.Context, ticketID, rawStatus string) (entity.SupportTicket, error)
}

type updateTicketStatusRequest struct {
	Status string `json:"status"`
}

func (handler *Handler) UpdateTicketStatusEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	ticketID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ticketID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TICKET_ID", "id must be a valid UUID")
		return
	}

	var body updateTicketStatusRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	ticket, err := handler.updateTicketStatusUseCase.Execute(request.Context(), ticketID, body.Status)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, ticketResponseFromEntity(ticket, handler.location))
}
