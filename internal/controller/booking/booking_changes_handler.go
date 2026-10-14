package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_changes_usecase_mock.go -package=mocks -source=./booking_changes_handler.go

// BookingChangesUseCase is what happens to a booking after it was made: the owner cancelling
// one they accepted, the driver extending it, and reading how to get into the spot.
// Implemented by usecase.BookingChangesUseCase.
type BookingChangesUseCase interface {
	OwnerCancel(ctx context.Context, bookingID, ownerID, reason string) (entity.Booking, error)
	ExtensionOptions(ctx context.Context, bookingID, driverID string) ([]entity.ExtensionOption, error)
	Extend(ctx context.Context, bookingID, driverID string, end time.Time) (entity.Booking, error)
	AccessInstructions(ctx context.Context, bookingID, viewerID string) (string, error)
	OwnerCancellationStats(ctx context.Context, ownerID string) (entity.OwnerCancellationStats, error)
}

type BookingChangesHandler struct {
	useCase  BookingChangesUseCase
	location *time.Location
}

func NewBookingChangesHandler(useCase BookingChangesUseCase, location *time.Location) *BookingChangesHandler {
	return &BookingChangesHandler{useCase: useCase, location: location}
}

func (handler *BookingChangesHandler) RegisterRoutes(router chi.Router) {
	router.Post("/bookings/{id}/owner-cancel", handler.ownerCancel)
	router.Get("/bookings/{id}/extension-options", handler.extensionOptions)
	router.Post("/bookings/{id}/extend", handler.extend)
	router.Get("/bookings/{id}/access", handler.accessInstructions)
	router.Get("/admin/owners/{id}/cancellation-stats", handler.ownerCancellationStats)
}

// requireUser is the caller's id, set by the gateway on every REQUIRED route.
func requireUser(responseWriter http.ResponseWriter, request *http.Request) (string, bool) {
	userID := request.Header.Get(userIDHeader)
	if userID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return "", false
	}
	return userID, true
}

type ownerCancelRequest struct {
	Reason string `json:"reason"`
}

func (handler *BookingChangesHandler) ownerCancel(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	ownerID, ok := requireUser(responseWriter, request)
	if !ok {
		return
	}
	var body ownerCancelRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	booking, err := handler.useCase.OwnerCancel(request.Context(), bookingID, ownerID, body.Reason)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, bookingResponseFromEntity(booking, handler.location, ""))
}

type extensionOptionResponse struct {
	End string `json:"end"`
	// What the driver pays for it, centavos.
	Amount int `json:"amount"`
}

func (handler *BookingChangesHandler) extensionOptions(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	driverID, ok := requireUser(responseWriter, request)
	if !ok {
		return
	}
	options, err := handler.useCase.ExtensionOptions(request.Context(), bookingID, driverID)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	items := make([]extensionOptionResponse, len(options))
	for index, option := range options {
		items[index] = extensionOptionResponse{End: option.End.In(handler.location).Format(time.RFC3339), Amount: option.Amount}
	}
	writeJSON(responseWriter, http.StatusOK, items)
}

type extendRequest struct {
	// RFC3339, one of extension-options' ends.
	End string `json:"end"`
}

func (handler *BookingChangesHandler) extend(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	driverID, ok := requireUser(responseWriter, request)
	if !ok {
		return
	}
	var body extendRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	end, err := time.Parse(time.RFC3339, body.End)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_END", "end must be an RFC3339 date-time")
		return
	}
	extension, err := handler.useCase.Extend(request.Context(), bookingID, driverID, end)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusCreated, bookingResponseFromEntity(extension, handler.location, driverID))
}

type accessInstructionsResponse struct {
	AccessInstructions string `json:"access_instructions"`
}

func (handler *BookingChangesHandler) accessInstructions(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	viewerID, ok := requireUser(responseWriter, request)
	if !ok {
		return
	}
	instructions, err := handler.useCase.AccessInstructions(request.Context(), bookingID, viewerID)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, accessInstructionsResponse{AccessInstructions: instructions})
}

type ownerCancellationStatsResponse struct {
	Total      int `json:"total"`
	Last90Days int `json:"last_90_days"`
}

func (handler *BookingChangesHandler) ownerCancellationStats(responseWriter http.ResponseWriter, request *http.Request) {
	ownerID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ownerID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "id must be a valid UUID")
		return
	}
	stats, err := handler.useCase.OwnerCancellationStats(request.Context(), ownerID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, ownerCancellationStatsResponse{Total: stats.Total, Last90Days: stats.Last90Days})
}
