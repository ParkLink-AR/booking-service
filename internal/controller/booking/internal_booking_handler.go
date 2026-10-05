package booking

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// internalBookingResponse is what payment-service needs to charge for a booking: whose it
// is, whether it's still awaiting payment, and the server-side price -- so the checkout
// amount never comes from the client.
type internalBookingResponse struct {
	ID            string  `json:"id"`
	UserID        string  `json:"user_id"`
	Status        string  `json:"status"`
	Amount        int     `json:"amount"`
	HoldExpiresAt *string `json:"hold_expires_at,omitempty"`
}

// GetInternalBookingEndpoint serves GET /internal/bookings/{id}, service-to-service only
// (X-Internal-Key; there's no gateway route for it).
func (handler *Handler) GetInternalBookingEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	if !handler.isInternalCaller(request) {
		writeErrorCode(responseWriter, http.StatusUnauthorized, "INVALID_INTERNAL_KEY", "invalid internal key")
		return
	}
	bookingID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(bookingID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BOOKING_ID", "id must be a valid UUID")
		return
	}

	booking, err := handler.getBookingUseCase.Execute(request.Context(), bookingID, entity.BookingViewer{Privileged: true})
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	response := internalBookingResponse{
		ID:     booking.ID,
		UserID: booking.UserID,
		Status: booking.Status.String(),
		Amount: booking.Amount,
	}
	if booking.HoldExpiresAt != nil {
		formatted := booking.HoldExpiresAt.UTC().Format(time.RFC3339)
		response.HoldExpiresAt = &formatted
	}
	writeJSON(responseWriter, http.StatusOK, response)
}
