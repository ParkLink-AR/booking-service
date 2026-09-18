package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/approve_booking_usecase_mock.go -package=mocks -source=./approve_booking_handler.go

// ApproveBookingUseCase is the interface consumed by this handler for POST /bookings/{id}/approve.
type ApproveBookingUseCase interface {
	Execute(ctx context.Context, bookingID, callerID string) (entity.Booking, error)
}

func (handler *Handler) ApproveBookingEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
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

	approved, err := handler.approveBookingUseCase.Execute(request.Context(), bookingID, callerID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND",
				"booking not found or not pending owner approval")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	// callerID is the owner here, not the driver -- bookingResponseFromEntity omits the
	// verification code whenever viewerID doesn't match the booking's own driver, so this
	// never leaks the code to the owner at the moment it's generated.
	writeJSON(responseWriter, http.StatusOK, bookingResponseFromEntity(approved, handler.location, callerID))
}
