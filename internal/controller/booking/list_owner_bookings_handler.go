package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_owner_bookings_usecase_mock.go -package=mocks -source=./list_owner_bookings_handler.go

// ListOwnerBookingsUseCase is the interface consumed by this handler for
// GET /admin/owners/{id}/bookings.
type ListOwnerBookingsUseCase interface {
	Execute(ctx context.Context, ownerID string) ([]entity.Booking, error)
}

func (handler *Handler) ListOwnerBookingsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	ownerID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ownerID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_OWNER_ID", "id must be a valid UUID")
		return
	}

	bookings, err := handler.listOwnerBookingsUseCase.Execute(request.Context(), ownerID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]ownerBookingResponse, len(bookings))
	for i, b := range bookings {
		responses[i] = ownerBookingResponseFromEntity(b, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
