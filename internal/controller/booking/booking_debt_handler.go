package booking

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_debt_usecase_mock.go -package=mocks -source=./booking_debt_handler.go

// BookingDebtUseCase is what drivers owe for staying past the end of a booking. Implemented by
// usecase.BookingDebtUseCase.
type BookingDebtUseCase interface {
	Mine(ctx context.Context, userID string) (entity.DebtSummary, error)
	ChargedTo(ctx context.Context, bookingID string) ([]entity.BookingDebt, error)
	OfBooking(ctx context.Context, bookingID string) ([]entity.BookingDebt, error)
}

type BookingDebtHandler struct {
	useCase            BookingDebtUseCase
	location           *time.Location
	internalServiceKey string
}

func NewBookingDebtHandler(useCase BookingDebtUseCase, location *time.Location, internalServiceKey string) *BookingDebtHandler {
	return &BookingDebtHandler{useCase: useCase, location: location, internalServiceKey: internalServiceKey}
}

func (handler *BookingDebtHandler) RegisterRoutes(router chi.Router) {
	router.Get("/debts", handler.mine)
	router.Get("/internal/bookings/{id}/debts", handler.chargedTo)
	router.Get("/admin/bookings/{id}/debts", handler.ofBooking)
}

// debtResponse is one debt as the driver (or support) sees it: what for, how long, how much and
// where it stands.
type debtResponse struct {
	ID string `json:"id"`
	// Status is PENDING (the next booking takes it on), RESERVED (it is on a booking waiting for
	// its payment) or PAID.
	Status string `json:"status"`
	// Amount owed, centavos, and the minutes past the end it is for.
	Amount  int `json:"amount"`
	Minutes int `json:"minutes"`
	// The booking that was overstayed, and where.
	BookingID        string `json:"booking_id"`
	BookingStart     string `json:"booking_start"`
	BookingEnd       string `json:"booking_end"`
	ParkingSpotID    string `json:"parking_spot_id"`
	ParkingSpotTitle string `json:"parking_spot_title"`
	// ChargedBookingID is the booking whose payment includes it, once one does.
	ChargedBookingID *string `json:"charged_booking_id,omitempty"`
	CreatedAt        string  `json:"created_at"`
	PaidAt           *string `json:"paid_at,omitempty"`
}

func (handler *BookingDebtHandler) debtResponses(debts []entity.BookingDebt) []debtResponse {
	responses := make([]debtResponse, 0, len(debts))
	for _, debt := range debts {
		response := debtResponse{
			ID:               debt.ID,
			Status:           debt.Status,
			Amount:           debt.Amount,
			Minutes:          debt.Minutes,
			BookingID:        debt.SourceBookingID,
			BookingStart:     debt.SourceStart.In(handler.location).Format(time.RFC3339),
			BookingEnd:       debt.SourceEnd.In(handler.location).Format(time.RFC3339),
			ParkingSpotID:    debt.ParkingSpotID,
			ParkingSpotTitle: debt.SpotTitle,
			ChargedBookingID: debt.ChargedBookingID,
			CreatedAt:        debt.CreatedAt.In(handler.location).Format(time.RFC3339),
		}
		if debt.PaidAt != nil {
			paidAt := debt.PaidAt.In(handler.location).Format(time.RFC3339)
			response.PaidAt = &paidAt
		}
		responses = append(responses, response)
	}
	return responses
}

type debtSummaryResponse struct {
	// PendingAmount is what a new booking would add to its price right now, centavos.
	PendingAmount int            `json:"pending_amount"`
	Debts         []debtResponse `json:"debts"`
}

type debtListResponse struct {
	Debts []debtResponse `json:"debts"`
}

func (handler *BookingDebtHandler) mine(responseWriter http.ResponseWriter, request *http.Request) {
	userID, ok := requireUser(responseWriter, request)
	if !ok {
		return
	}
	summary, err := handler.useCase.Mine(request.Context(), userID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, debtSummaryResponse{
		PendingAmount: summary.PendingAmount,
		Debts:         handler.debtResponses(summary.Debts),
	})
}

// chargedTo serves GET /internal/bookings/{id}/debts, service-to-service only (X-Internal-Key;
// there is no gateway route for it): what payment-service words a checkout with.
func (handler *BookingDebtHandler) chargedTo(responseWriter http.ResponseWriter, request *http.Request) {
	providedKey := request.Header.Get(internalKeyHeader)
	if handler.internalServiceKey == "" ||
		subtle.ConstantTimeCompare([]byte(providedKey), []byte(handler.internalServiceKey)) != 1 {
		writeErrorCode(responseWriter, http.StatusUnauthorized, "INVALID_INTERNAL_KEY", "invalid internal key")
		return
	}
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	debts, err := handler.useCase.ChargedTo(request.Context(), bookingID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, debtListResponse{Debts: handler.debtResponses(debts)})
}

func (handler *BookingDebtHandler) ofBooking(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	debts, err := handler.useCase.OfBooking(request.Context(), bookingID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, debtListResponse{Debts: handler.debtResponses(debts)})
}
