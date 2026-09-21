package booking

import (
	"context"
	"net/http"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_tickets_usecase_mock.go -package=mocks -source=./list_admin_tickets_handler.go

// ListAdminTicketsUseCase is the interface consumed by this handler for GET /admin/tickets.
type ListAdminTicketsUseCase interface {
	Execute(ctx context.Context, status *enum.TicketStatus) ([]entity.SupportTicket, error)
}

func (handler *Handler) ListAdminTicketsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	var status *enum.TicketStatus
	if raw := request.URL.Query().Get("status"); raw != "" {
		parsed, ok := enum.ParseTicketStatus(raw)
		if !ok {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_STATUS",
				"status must be one of OPEN, IN_PROGRESS, RESOLVED, CLOSED")
			return
		}
		status = &parsed
	}

	tickets, err := handler.listAdminTicketsUseCase.Execute(request.Context(), status)
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
