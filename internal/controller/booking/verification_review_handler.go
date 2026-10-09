package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/verification_review_usecase_mock.go -package=mocks -source=./verification_review_handler.go

// ListPendingVerificationReviewsUseCase is the interface consumed by this handler for
// GET /admin/bookings/unverified.
type ListPendingVerificationReviewsUseCase interface {
	Execute(ctx context.Context) ([]entity.Booking, error)
}

// ResolveVerificationReviewUseCase is the interface consumed by this handler for
// POST /admin/bookings/{id}/verification-review.
type ResolveVerificationReviewUseCase interface {
	Execute(ctx context.Context, bookingID string, review enum.VerificationReview) (entity.Booking, error)
}

type resolveVerificationReviewRequest struct {
	// Decision is "APPROVED" (the owner gets paid) or "REFUNDED" (the driver gets their
	// money back).
	Decision string `json:"decision"`
}

// verificationReviewResponse is one row of the admin panel's "reservas sin verificar"
// queue: a paid, completed booking whose arrival code the owner never entered.
type verificationReviewResponse struct {
	ID                 string           `json:"id"`
	ParkingSpotID      string           `json:"parking_spot_id"`
	UserID             string           `json:"user_id"`
	Start              string           `json:"start"`
	End                string           `json:"end"`
	Status             string           `json:"status"`
	Kind               string           `json:"kind"`
	Amount             int              `json:"amount"`
	OwnerAmount        *int             `json:"owner_amount,omitempty"`
	PaymentID          string           `json:"payment_id,omitempty"`
	PaidAt             *string          `json:"paid_at,omitempty"`
	VerificationReview string           `json:"verification_review"`
	UpdatedAt          string           `json:"updated_at"`
	Vehicle            *vehicleResponse `json:"vehicle,omitempty"`
}

func verificationReviewResponseFromEntity(booking entity.Booking, location *time.Location) verificationReviewResponse {
	response := verificationReviewResponse{
		ID:            booking.ID,
		ParkingSpotID: booking.ParkingSpotID,
		UserID:        booking.UserID,
		Start:         booking.Start.In(location).Format(time.RFC3339),
		End:           booking.End.In(location).Format(time.RFC3339),
		Status:        booking.Status.String(),
		Kind:          booking.Kind.String(),
		Amount:        booking.Amount,
		OwnerAmount:   booking.OwnerAmount,
		PaymentID:     booking.PaymentID,
		UpdatedAt:     booking.UpdatedAt.In(location).Format(time.RFC3339),
		Vehicle:       vehicleResponseFromEntity(booking),
	}
	if booking.PaidAt != nil {
		formatted := booking.PaidAt.In(location).Format(time.RFC3339)
		response.PaidAt = &formatted
	}
	if booking.VerificationReview != nil {
		response.VerificationReview = booking.VerificationReview.String()
	}
	return response
}

func (handler *Handler) ListPendingVerificationReviewsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	bookings, err := handler.listPendingVerificationReviewsUseCase.Execute(request.Context())
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]verificationReviewResponse, len(bookings))
	for i, booking := range bookings {
		responses[i] = verificationReviewResponseFromEntity(booking, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}

func (handler *Handler) ResolveVerificationReviewEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(bookingID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BOOKING_ID", "id must be a valid UUID")
		return
	}

	var body resolveVerificationReviewRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	review, ok := enum.ParseVerificationReview(body.Decision)
	if !ok || review == enum.VerificationReviewPending {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_DECISION", "decision must be APPROVED or REFUNDED")
		return
	}

	resolved, err := handler.resolveVerificationReviewUseCase.Execute(request.Context(), bookingID, review)
	if err != nil {
		if errors.IsConflict(err) {
			writeErrorCode(responseWriter, http.StatusConflict, "REVIEW_NOT_PENDING", err.Error())
			return
		}
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, verificationReviewResponseFromEntity(resolved, handler.location))
}
