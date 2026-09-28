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

//go:generate go tool mockgen -destination=../../../pkg/mocks/confirm_settlement_usecase_mock.go -package=mocks -source=./confirm_settlement_handler.go

// maxProofUploadBytes caps how much of the multipart form (and therefore the proof image)
// this endpoint will buffer into memory.
const maxProofUploadBytes = 10 << 20 // 10MB

// ConfirmSettlementUseCase is the interface consumed by this handler for
// POST /admin/settlements/{id}/confirm.
type ConfirmSettlementUseCase interface {
	Execute(ctx context.Context, settlementID, adminUserID string, proof []byte, proofContentType string) (entity.Settlement, []entity.Payout, error)
}

// ConfirmSettlementEndpoint is phase 2 of settling an owner's payouts (see entity.Settlement):
// this is the step that asserts the bank transfer actually went out.
func (handler *Handler) ConfirmSettlementEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	settlementID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(settlementID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SETTLEMENT_ID", "id must be a valid UUID")
		return
	}

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
		// No proof attached -- allowed, ConfirmSettlementUseCase confirms without one.
	default:
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_PROOF", "could not read proof file")
		return
	}

	settlement, payouts, err := handler.confirmSettlementUseCase.Execute(request.Context(), settlementID, adminUserID, proof, proofContentType)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "SETTLEMENT_NOT_FOUND", "settlement not found")
			return
		}
		if errors.IsConflict(err) {
			writeErrorCode(responseWriter, http.StatusConflict, "ALREADY_CONFIRMED", err.Error())
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, settlementResponseFromEntity(settlement, payouts, handler.location))
}
