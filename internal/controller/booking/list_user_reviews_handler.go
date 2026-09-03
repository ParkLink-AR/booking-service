package booking

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/enum"
)

func (handler *Handler) ListUserReviewsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	userID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(userID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "id must be a valid UUID")
		return
	}

	as := request.URL.Query().Get("as")
	targetType, ok := enum.ParseReviewTargetType(as)
	if !ok || targetType == enum.ReviewTargetSpot {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_AS", "as must be owner or driver")
		return
	}

	reviews, err := handler.listReviewsByTargetUseCase.Execute(request.Context(), targetType, userID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, reviewListResponseFromEntities(reviews, handler.location))
}
