package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/create_ticket_usecase_mock.go -package=mocks -source=./create_ticket_handler.go

// CreateTicketUseCase is the interface consumed by this handler for POST /bookings/{id}/tickets.
type CreateTicketUseCase interface {
	Execute(ctx context.Context, bookingID, callerID, subject, body string) (entity.SupportTicket, error)
}

type createTicketRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type ticketResponse struct {
	ID        string `json:"id"`
	BookingID string `json:"booking_id"`
	CreatedBy string `json:"created_by"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func ticketResponseFromEntity(ticket entity.SupportTicket, location *time.Location) ticketResponse {
	return ticketResponse{
		ID:        ticket.ID,
		BookingID: ticket.BookingID,
		CreatedBy: ticket.CreatedBy,
		Subject:   ticket.Subject,
		Status:    ticket.Status.String(),
		CreatedAt: ticket.CreatedAt.In(location).Format(time.RFC3339),
		UpdatedAt: ticket.UpdatedAt.In(location).Format(time.RFC3339),
	}
}

type ticketMessageResponse struct {
	ID        string `json:"id"`
	TicketID  string `json:"ticket_id"`
	SenderID  string `json:"sender_id"`
	IsAdmin   bool   `json:"is_admin"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func ticketMessageResponseFromEntity(message entity.SupportTicketMessage, location *time.Location) ticketMessageResponse {
	return ticketMessageResponse{
		ID:        message.ID,
		TicketID:  message.TicketID,
		SenderID:  message.SenderID,
		IsAdmin:   message.IsAdmin,
		Body:      message.Body,
		CreatedAt: message.CreatedAt.In(location).Format(time.RFC3339),
	}
}

func (handler *Handler) CreateTicketEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
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

	var body createTicketRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	ticket, err := handler.createTicketUseCase.Execute(request.Context(), bookingID, callerID, body.Subject, body.Body)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusCreated, ticketResponseFromEntity(ticket, handler.location))
}
