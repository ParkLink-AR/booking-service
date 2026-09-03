package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_bookings_detailed_usecase_mock.go -package=mocks -source=./list_bookings_detailed_handler.go

// ListBookingsDetailedUseCase is the interface consumed by this handler for
// GET /parking-spots/{id}/bookings/detailed.
type ListBookingsDetailedUseCase interface {
	Execute(ctx context.Context, spotID, callerID string) ([]entity.Booking, error)
}

func (handler *Handler) ListBookingsDetailedEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	parkingSpotID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(parkingSpotID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "id must be a valid UUID")
		return
	}

	callerID := request.Header.Get("X-User-Id")
	if callerID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	bookings, err := handler.listBookingsDetailedUseCase.Execute(request.Context(), parkingSpotID, callerID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]*bookingResponse, len(bookings))
	for i, booking := range bookings {
		responses[i] = bookingResponseFromEntity(booking, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
