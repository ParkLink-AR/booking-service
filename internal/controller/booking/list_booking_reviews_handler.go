package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_reviews_by_booking_usecase_mock.go -package=mocks -source=./list_booking_reviews_handler.go

// ListReviewsByBookingUseCase is the interface consumed by this handler for
// GET /bookings/{id}/reviews.
type ListReviewsByBookingUseCase interface {
	Execute(ctx context.Context, bookingID string) ([]entity.Review, error)
}

func (handler *Handler) ListBookingReviewsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(bookingID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BOOKING_ID", "id must be a valid UUID")
		return
	}

	reviews, err := handler.listReviewsByBookingUseCase.Execute(request.Context(), bookingID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]reviewResponse, len(reviews))
	for i, review := range reviews {
		responses[i] = reviewResponseFromEntity(review, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}
