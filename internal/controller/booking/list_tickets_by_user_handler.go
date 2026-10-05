package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_tickets_by_user_usecase_mock.go -package=mocks -source=./list_tickets_by_user_handler.go

// ticketSummaryResponse is the wire shape of a TicketSummary for GET /users/{id}/tickets.
type ticketSummaryResponse struct {
	ID               string  `json:"id"`
	BookingID        string  `json:"booking_id"`
	CreatedBy        string  `json:"created_by"`
	Subject          string  `json:"subject"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	LastAdminReplyID *string `json:"last_admin_reply_id,omitempty"`
	LastAdminReplyAt *string `json:"last_admin_reply_at,omitempty"`
}

func ticketSummaryResponseFromEntity(summary entity.TicketSummary, location *time.Location) ticketSummaryResponse {
	response := ticketSummaryResponse{
		ID:               summary.ID,
		BookingID:        summary.BookingID,
		CreatedBy:        summary.CreatedBy,
		Subject:          summary.Subject,
		Status:           summary.Status.String(),
		CreatedAt:        summary.CreatedAt.In(location).Format(time.RFC3339),
		UpdatedAt:        summary.UpdatedAt.In(location).Format(time.RFC3339),
		LastAdminReplyID: summary.LastAdminReplyID,
	}
	if summary.LastAdminReplyAt != nil {
		formatted := summary.LastAdminReplyAt.In(location).Format(time.RFC3339)
		response.LastAdminReplyAt = &formatted
	}
	return response
}

// ListTicketsByUserUseCase is the interface consumed by this handler for GET /users/{id}/tickets.
type ListTicketsByUserUseCase interface {
	Execute(ctx context.Context, userID string) ([]entity.TicketSummary, error)
}

func (handler *Handler) ListTicketsByUserEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	userID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(userID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "id must be a valid UUID")
		return
	}
	if !requireSelfOrAdmin(responseWriter, request, userID) {
		return
	}

	summaries, err := handler.listTicketsByUserUseCase.Execute(request.Context(), userID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]ticketSummaryResponse, len(summaries))
	for i, summary := range summaries {
		responses[i] = ticketSummaryResponseFromEntity(summary, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
