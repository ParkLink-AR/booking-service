package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/submit_review_usecase_mock.go -package=mocks -source=./submit_review_handler.go

// SubmitReviewUseCase is the interface consumed by this handler for POST /bookings/{id}/reviews.
type SubmitReviewUseCase interface {
	Execute(ctx context.Context, bookingID, callerID string, targetType enum.ReviewTargetType, rating int, comment string) (entity.Review, error)
}

type submitReviewRequest struct {
	TargetType string `json:"targetType"`
	Rating     int    `json:"rating"`
	Comment    string `json:"comment"`
}

func (handler *Handler) SubmitReviewEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
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

	var body submitReviewRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	targetType, ok := enum.ParseReviewTargetType(body.TargetType)
	if !ok {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TARGET_TYPE", "targetType must be spot, owner, or driver")
		return
	}

	review, err := handler.submitReviewUseCase.Execute(request.Context(), bookingID, callerID, targetType, body.Rating, body.Comment)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusCreated, reviewResponseFromEntity(review, handler.location))
}
