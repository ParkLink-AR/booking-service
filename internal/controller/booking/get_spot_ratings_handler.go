package booking

import (
	"context"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_spot_ratings_usecase_mock.go -package=mocks -source=./get_spot_ratings_handler.go

// GetSpotRatingsUseCase is the interface consumed by this handler for
// POST /parking-spots/ratings. Not gateway-routed -- parking-service calls this
// directly, same as it does for POST /parking-spots/bookable.
type GetSpotRatingsUseCase interface {
	Execute(ctx context.Context, spotIDs []string) (map[string]entity.RatingSummary, error)
}

type getSpotRatingsRequest struct {
	ParkingSpotIDs []string `json:"parking_spot_ids"`
}

type spotRatingResponse struct {
	AverageRating float64 `json:"average_rating"`
	Count         int     `json:"count"`
}

type getSpotRatingsResponse struct {
	Ratings map[string]spotRatingResponse `json:"ratings"`
}

func (handler *Handler) GetSpotRatingsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	var body getSpotRatingsRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	for _, spotID := range body.ParkingSpotIDs {
		if _, err := uuid.Parse(spotID); err != nil {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "parking_spot_ids must all be valid UUIDs")
			return
		}
	}

	ratings, err := handler.getSpotRatingsUseCase.Execute(request.Context(), body.ParkingSpotIDs)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	response := getSpotRatingsResponse{Ratings: make(map[string]spotRatingResponse, len(ratings))}
	for spotID, summary := range ratings {
		response.Ratings[spotID] = spotRatingResponse{AverageRating: summary.Average, Count: summary.Count}
	}
	writeJSON(responseWriter, http.StatusOK, response)
}
