package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/cancel_booking_usecase_mock.go -package=mocks -source=./cancel_booking_handler.go

// CancelBookingUseCase is the interface consumed by this handler for POST /bookings/{id}/cancel.
type CancelBookingUseCase interface {
	Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error)
}

func (handler *Handler) CancelBookingEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
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

	cancelled, err := handler.cancelBookingUseCase.Execute(request.Context(), bookingID, callerID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND",
				"booking not found or not in a cancellable status")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, bookingResponseFromEntity(cancelled, handler.location, callerID))
}
