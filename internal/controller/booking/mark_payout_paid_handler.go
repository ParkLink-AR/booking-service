package booking

import (
	"context"
	stderrors "errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/mark_payout_paid_usecase_mock.go -package=mocks -source=./mark_payout_paid_handler.go

// maxProofUploadBytes caps how much of the multipart form (and therefore the proof image)
// this endpoint will buffer into memory.
const maxProofUploadBytes = 10 << 20 // 10MB

// MarkPayoutPaidUseCase is the interface consumed by this handler for
// POST /admin/payouts/{id}/mark-paid.
type MarkPayoutPaidUseCase interface {
	Execute(ctx context.Context, payoutID, adminUserID string, proof []byte, proofContentType string) (entity.Payout, error)
}

func (handler *Handler) MarkPayoutPaidEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	payoutID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(payoutID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_PAYOUT_ID", "id must be a valid UUID")
		return
	}

	// The gateway's admin auth mode forwards X-User-Id (the admin JWT's sub claim) the same
	// way it does for /admin/tickets/* -- reused here to record who marked the payout paid.
	adminUserID := request.Header.Get("X-User-Id")
	if adminUserID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}

	if err := request.ParseMultipartForm(maxProofUploadBytes); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request must be multipart/form-data")
		return
	}

	var proof []byte
	var proofContentType string
	file, header, err := request.FormFile("proof")
	switch {
	case err == nil:
		defer file.Close()
		data, readErr := io.ReadAll(file)
		if readErr != nil {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_PROOF", "could not read proof file")
			return
		}
		proof = data
		proofContentType = header.Header.Get("Content-Type")
	case stderrors.Is(err, http.ErrMissingFile):
		// No proof attached -- allowed, MarkPayoutPaidUseCase marks paid without one.
	default:
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_PROOF", "could not read proof file")
		return
	}

	payout, err := handler.markPayoutPaidUseCase.Execute(request.Context(), payoutID, adminUserID, proof, proofContentType)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "PAYOUT_NOT_FOUND", "payout not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, payoutResponseFromEntity(payout, handler.location))
}
