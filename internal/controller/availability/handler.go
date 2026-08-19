package availability

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_availability_usecase_mock.go -package=mocks -source=./handler.go

const dateLayout = "2006-01-02"

// GetAvailabilityUseCase is the interface consumed by this controller.
type GetAvailabilityUseCase interface {
	Execute(ctx context.Context, parkingSpotID string, from, to time.Time) ([]entity.DayAvailability, error)
}

// Handler handles HTTP requests for parking spot availability.
type Handler struct {
	useCase  GetAvailabilityUseCase
	location *time.Location
}

func NewHandler(useCase GetAvailabilityUseCase, location *time.Location) *Handler {
	return &Handler{useCase: useCase, location: location}
}

func (handler *Handler) RegisterRoutes(router chi.Router) {
	router.Get(parkingSpotsBasePath+"/{id}/availability", handler.GetAvailabilityEndpoint)
}

func (handler *Handler) GetAvailabilityEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	parkingSpotID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(parkingSpotID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "id must be a valid UUID")
		return
	}

	from, err := time.ParseInLocation(dateLayout, request.URL.Query().Get("from"), handler.location)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_DATE", "from must be a valid YYYY-MM-DD date")
		return
	}
	to, err := time.ParseInLocation(dateLayout, request.URL.Query().Get("to"), handler.location)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_DATE", "to must be a valid YYYY-MM-DD date")
		return
	}

	days, err := handler.useCase.Execute(request.Context(), parkingSpotID, from, to)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, availabilityResponseFromEntities(days, handler.location))
}

// writeDomainError translates domain errors to HTTP status codes and machine-readable codes.
// HTTP knowledge lives only in the controller — no other layer imports net/http.
func writeDomainError(responseWriter http.ResponseWriter, err error) {
	switch {
	case errors.IsInvalidInput(err):
		writeErrorCode(responseWriter, http.StatusBadRequest, errors.InvalidInputCode(err), err.Error())
	case errors.IsNotFound(err):
		writeErrorCode(responseWriter, http.StatusNotFound, "SPOT_NOT_FOUND", err.Error())
	case errors.IsUpstream(err):
		writeErrorCode(responseWriter, http.StatusBadGateway, "PARKING_SERVICE_ERROR", err.Error())
	default:
		writeErrorCode(responseWriter, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
