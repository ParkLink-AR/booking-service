package booking

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/confirm_booking_usecase_mock.go -package=mocks -source=./confirm_booking_handler.go

// ConfirmBookingUseCase is the interface consumed by this handler for POST /bookings/{id}/confirm.
type ConfirmBookingUseCase interface {
	Execute(ctx context.Context, bookingID, paymentID string) (entity.Booking, error)
}

type confirmBookingRequest struct {
	PaymentID string `json:"payment_id"`
}

func (handler *Handler) ConfirmBookingEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	// Only payment-service may confirm a booking -- and only once Mercado Pago has
	// approved its payment. Without this any logged-in user could mark their own booking
	// as paid with a made-up payment_id. An unset key rejects every call (fail closed).
	providedKey := request.Header.Get(internalKeyHeader)
	if handler.internalServiceKey == "" ||
		subtle.ConstantTimeCompare([]byte(providedKey), []byte(handler.internalServiceKey)) != 1 {
		writeErrorCode(responseWriter, http.StatusUnauthorized, "INVALID_INTERNAL_KEY", "invalid internal key")
		return
	}

	bookingID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(bookingID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BOOKING_ID", "id must be a valid UUID")
		return
	}

	var body confirmBookingRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	if body.PaymentID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_PAYMENT_ID", "payment_id is required")
		return
	}

	confirmed, err := handler.confirmBookingUseCase.Execute(request.Context(), bookingID, body.PaymentID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND",
				"booking not found, already confirmed, or hold expired")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	// This response only ever goes back to payment-service (see the internal key check
	// above), so it's safe to always include the fresh verification code here.
	writeJSON(responseWriter, http.StatusOK, bookingResponseFromEntity(confirmed, handler.location, confirmed.UserID))
}
