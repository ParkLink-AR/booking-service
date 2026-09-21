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

//go:generate go tool mockgen -destination=../../../pkg/mocks/send_message_usecase_mock.go -package=mocks -source=./send_message_handler.go

// SendMessageUseCase is the interface consumed by this handler for POST /bookings/{id}/messages.
type SendMessageUseCase interface {
	Execute(ctx context.Context, bookingID, callerID, body string) (entity.BookingMessage, error)
}

type sendMessageRequest struct {
	Body string `json:"body"`
}

type messageResponse struct {
	ID        string `json:"id"`
	BookingID string `json:"booking_id"`
	SenderID  string `json:"sender_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func messageResponseFromEntity(message entity.BookingMessage, location *time.Location) messageResponse {
	return messageResponse{
		ID:        message.ID,
		BookingID: message.BookingID,
		SenderID:  message.SenderID,
		Body:      message.Body,
		CreatedAt: message.CreatedAt.In(location).Format(time.RFC3339),
	}
}

func (handler *Handler) SendMessageEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
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

	var body sendMessageRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	message, err := handler.sendMessageUseCase.Execute(request.Context(), bookingID, callerID, body.Body)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusCreated, messageResponseFromEntity(message, handler.location))
}
