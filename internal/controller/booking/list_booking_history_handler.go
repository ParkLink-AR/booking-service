package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_booking_history_usecase_mock.go -package=mocks -source=./list_booking_history_handler.go

// ListBookingHistoryUseCase is the interface consumed by this controller for
// GET /users/{id}/bookings/history.
type ListBookingHistoryUseCase interface {
	Execute(ctx context.Context, userID string) ([]entity.Booking, error)
}

type bookingHistoryResponse struct {
	Bookings []*bookingResponse `json:"bookings"`
}

func (handler *Handler) ListBookingHistoryEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	userID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(userID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "id must be a valid UUID")
		return
	}

	bookings, err := handler.listBookingHistoryUseCase.Execute(request.Context(), userID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	response := bookingHistoryResponse{Bookings: make([]*bookingResponse, len(bookings))}
	for index, booking := range bookings {
		response.Bookings[index] = bookingResponseFromEntity(booking, handler.location)
	}

	writeJSON(responseWriter, http.StatusOK, response)
}
