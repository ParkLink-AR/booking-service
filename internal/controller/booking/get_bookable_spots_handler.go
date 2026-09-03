package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_bookable_spots_usecase_mock.go -package=mocks -source=./get_bookable_spots_handler.go

// GetBookableSpotsUseCase is the interface consumed by this controller.
type GetBookableSpotsUseCase interface {
	Execute(ctx context.Context, spots []entity.SpotCapacity, start, end time.Time) ([]string, error)
}

type spotCapacityRequest struct {
	ID       string `json:"id"`
	Capacity int    `json:"capacity"`
}

type getBookableSpotsRequest struct {
	Spots []spotCapacityRequest `json:"spots"`
	Start string                `json:"start"`
	End   string                `json:"end"`
}

type getBookableSpotsResponse struct {
	BookableParkingSpotIDs []string `json:"bookable_parking_spot_ids"`
}

func (handler *Handler) GetBookableSpotsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	var body getBookableSpotsRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	if len(body.Spots) == 0 {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "spots must not be empty")
		return
	}

	spots := make([]entity.SpotCapacity, len(body.Spots))
	for index, spot := range body.Spots {
		if _, err := uuid.Parse(spot.ID); err != nil {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "spots[].id must all be valid UUIDs")
			return
		}
		capacity := spot.Capacity
		if capacity < 1 {
			capacity = 1
		}
		spots[index] = entity.SpotCapacity{ID: spot.ID, Capacity: capacity}
	}

	start, err := time.Parse(time.RFC3339, body.Start)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TIME_RANGE", "start must be a valid RFC3339 timestamp")
		return
	}
	end, err := time.Parse(time.RFC3339, body.End)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TIME_RANGE", "end must be a valid RFC3339 timestamp")
		return
	}

	bookableParkingSpotIDs, err := handler.getBookableSpotsUseCase.Execute(request.Context(), spots, start, end)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, getBookableSpotsResponse{BookableParkingSpotIDs: bookableParkingSpotIDs})
}
