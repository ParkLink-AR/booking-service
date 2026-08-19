package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_bookable_spots_usecase_mock.go -package=mocks -source=./get_bookable_spots_handler.go

// GetBookableSpotsUseCase is the interface consumed by this controller.
type GetBookableSpotsUseCase interface {
	Execute(ctx context.Context, parkingSpotIDs []string, start, end time.Time) ([]string, error)
}

type getBookableSpotsRequest struct {
	ParkingSpotIDs []string `json:"parking_spot_ids"`
	Start          string   `json:"start"`
	End            string   `json:"end"`
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

	if len(body.ParkingSpotIDs) == 0 {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "parking_spot_ids must not be empty")
		return
	}
	for _, parkingSpotID := range body.ParkingSpotIDs {
		if _, err := uuid.Parse(parkingSpotID); err != nil {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "parking_spot_ids must all be valid UUIDs")
			return
		}
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

	bookableParkingSpotIDs, err := handler.getBookableSpotsUseCase.Execute(request.Context(), body.ParkingSpotIDs, start, end)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, getBookableSpotsResponse{BookableParkingSpotIDs: bookableParkingSpotIDs})
}
