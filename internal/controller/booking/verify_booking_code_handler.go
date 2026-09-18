package booking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/verify_booking_code_usecase_mock.go -package=mocks -source=./verify_booking_code_handler.go

// VerifyBookingCodeUseCase is the interface consumed by this handler for
// POST /bookings/{id}/verify-code.
type VerifyBookingCodeUseCase interface {
	Execute(ctx context.Context, bookingID, callerID, code string) (entity.Booking, error)
}

type verifyBookingCodeRequest struct {
	Code string `json:"code"`
}

func (handler *Handler) VerifyBookingCodeEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
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

	var body verifyBookingCodeRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	if len(body.Code) != 4 {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_CODE", "code must be 4 digits")
		return
	}

	verified, err := handler.verifyBookingCodeUseCase.Execute(request.Context(), bookingID, callerID, body.Code)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found or not confirmed")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	// The owner is the one calling this endpoint -- callerID is the owner, never the
	// driver, so this response never includes the (now-consumed) verification code.
	writeJSON(responseWriter, http.StatusOK, bookingResponseFromEntity(verified, handler.location, ""))
}
