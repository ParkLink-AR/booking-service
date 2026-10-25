package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/owner_earnings_usecase_mock.go -package=mocks -source=./owner_earnings_handler.go

// OwnerEarningsUseCase is the interface consumed by this handler for GET /users/{id}/earnings.
// Implemented by usecase.GetOwnerEarningsUseCase.
type OwnerEarningsUseCase interface {
	Execute(ctx context.Context, ownerID string) (entity.OwnerEarnings, error)
}

type OwnerEarningsHandler struct {
	useCase OwnerEarningsUseCase
}

func NewOwnerEarningsHandler(useCase OwnerEarningsUseCase) *OwnerEarningsHandler {
	return &OwnerEarningsHandler{useCase: useCase}
}

func (handler *OwnerEarningsHandler) RegisterRoutes(router chi.Router) {
	router.Get("/users/{id}/earnings", handler.ownerEarnings)
}

type monthEarningsResponse struct {
	Month    string `json:"month"`
	Earned   int    `json:"earned"`
	Bookings int    `json:"bookings"`
}

type spotEarningsResponse struct {
	ParkingSpotID string `json:"parking_spot_id"`
	Earned        int    `json:"earned"`
	Bookings      int    `json:"bookings"`
}

type ownerEarningsResponse struct {
	Months        []monthEarningsResponse `json:"months"`
	Spots         []spotEarningsResponse  `json:"spots"`
	TotalEarned   int                     `json:"total_earned"`
	PaidOut       int                     `json:"paid_out"`
	ToCollect     int                     `json:"to_collect"`
	Upcoming      int                     `json:"upcoming"`
	UpcomingCount int                     `json:"upcoming_count"`
}

func ownerEarningsResponseFromEntity(earnings entity.OwnerEarnings) ownerEarningsResponse {
	months := make([]monthEarningsResponse, len(earnings.Months))
	for i, month := range earnings.Months {
		months[i] = monthEarningsResponse{Month: month.Month, Earned: month.Earned, Bookings: month.Bookings}
	}
	spots := make([]spotEarningsResponse, len(earnings.Spots))
	for i, spot := range earnings.Spots {
		spots[i] = spotEarningsResponse{ParkingSpotID: spot.ParkingSpotID, Earned: spot.Earned, Bookings: spot.Bookings}
	}
	return ownerEarningsResponse{
		Months:        months,
		Spots:         spots,
		TotalEarned:   earnings.TotalEarned,
		PaidOut:       earnings.PaidOut,
		ToCollect:     earnings.ToCollect,
		Upcoming:      earnings.Upcoming,
		UpcomingCount: earnings.UpcomingCount,
	}
}

// ownerEarnings is the caller's own earnings (or any owner's, for ParkLink staff).
func (handler *OwnerEarningsHandler) ownerEarnings(responseWriter http.ResponseWriter, request *http.Request) {
	ownerID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ownerID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "id must be a valid UUID")
		return
	}
	if !requireSelfOrAdmin(responseWriter, request, ownerID) {
		return
	}
	earnings, err := handler.useCase.Execute(request.Context(), ownerID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, ownerEarningsResponseFromEntity(earnings))
}
