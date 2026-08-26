package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_booking_usecase_mock.go -package=mocks -source=./get_booking_handler.go

// GetBookingUseCase is the interface consumed by this controller for GET /bookings/{id}.
type GetBookingUseCase interface {
	Execute(ctx context.Context, bookingID string) (entity.Booking, error)
}

func (handler *Handler) GetBookingEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(bookingID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BOOKING_ID", "id must be a valid UUID")
		return
	}

	found, err := handler.getBookingUseCase.Execute(request.Context(), bookingID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, bookingResponseFromEntity(found, handler.location))
}
