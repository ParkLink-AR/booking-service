package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_reviews_by_target_usecase_mock.go -package=mocks -source=./list_spot_reviews_handler.go

// ListReviewsByTargetUseCase is the interface consumed by the two handlers in this
// file and in list_user_reviews_handler.go.
type ListReviewsByTargetUseCase interface {
	Execute(ctx context.Context, targetType enum.ReviewTargetType, targetID string) ([]entity.Review, error)
}

func (handler *Handler) ListSpotReviewsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	parkingSpotID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(parkingSpotID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "id must be a valid UUID")
		return
	}

	reviews, err := handler.listReviewsByTargetUseCase.Execute(request.Context(), enum.ReviewTargetSpot, parkingSpotID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, reviewListResponseFromEntities(reviews, handler.location))
}
