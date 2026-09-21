package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_messages_usecase_mock.go -package=mocks -source=./list_messages_handler.go

// ListMessagesUseCase is the interface consumed by this handler for GET /bookings/{id}/messages.
type ListMessagesUseCase interface {
	Execute(ctx context.Context, bookingID, callerID string) ([]entity.BookingMessage, error)
}

func (handler *Handler) ListMessagesEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
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

	messages, err := handler.listMessagesUseCase.Execute(request.Context(), bookingID, callerID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]messageResponse, len(messages))
	for i, message := range messages {
		responses[i] = messageResponseFromEntity(message, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
