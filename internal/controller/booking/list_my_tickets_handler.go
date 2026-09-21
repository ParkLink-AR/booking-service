package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_my_tickets_usecase_mock.go -package=mocks -source=./list_my_tickets_handler.go

// ListMyTicketsUseCase is the interface consumed by this handler for GET /bookings/{id}/tickets.
type ListMyTicketsUseCase interface {
	Execute(ctx context.Context, bookingID, callerID string) ([]entity.SupportTicket, error)
}

func (handler *Handler) ListMyTicketsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(bookingID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BOOKING_ID", "id must be a valid UUID")
		return
	}

	callerID := request.Header.Get("X-User-Id")
	if callerID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	tickets, err := handler.listMyTicketsUseCase.Execute(request.Context(), bookingID, callerID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]ticketResponse, len(tickets))
	for i, ticket := range tickets {
		responses[i] = ticketResponseFromEntity(ticket, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
