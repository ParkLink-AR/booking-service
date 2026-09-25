package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/mark_payout_paid_repository_mock.go -package=mocks -source=./mark_payout_paid_usecase.go

// PayoutMarker transitions a payout PENDING -> PAID. Implemented by
// internal/repository/booking; returns a ConflictError if the payout is already PAID and a
// NotFoundError if it doesn't exist.
type PayoutMarker interface {
	MarkPayoutPaid(ctx context.Context, payoutID, adminUserID string, proofURL *string) (entity.Payout, error)
}

// StorageService uploads a payout's bank-transfer proof image and returns its public URL.
// Implemented by internal/repository/storage (Cloudflare R2).
type StorageService interface {
	UploadPayoutProof(ctx context.Context, ownerID, paymentID, contentType string, data []byte) (string, error)
}

// PayoutByIDGetter fetches a single payout by id -- used here to resolve ownerID/paymentID
// for the proof upload before the payout is marked paid. Implemented by
// internal/repository/booking.
type PayoutByIDGetter interface {
	GetPayoutByID(ctx context.Context, payoutID string) (entity.Payout, error)
}

// MarkPayoutPaidUseCase powers POST /admin/payouts/{id}/mark-paid. If a proof image is
// supplied, it's uploaded first -- a failed upload aborts the whole call, so a payout is
// never marked PAID without the proof the admin thought they were attaching. Once marked
// PAID, it publishes payout.paid to notify the owner; unlike the proof upload, a failed
// publish does NOT fail the call (see emitBookingEvent).
type MarkPayoutPaidUseCase struct {
	payoutGetter PayoutByIDGetter
	payouts      PayoutMarker
	bookings     BookingGetter
	spots        SpotDetailsLookup
	storage      StorageService
	events       EventPublisher
}

func NewMarkPayoutPaidUseCase(
	payoutGetter PayoutByIDGetter,
	payouts PayoutMarker,
	bookings BookingGetter,
	spots SpotDetailsLookup,
	storage StorageService,
	events EventPublisher,
) *MarkPayoutPaidUseCase {
	return &MarkPayoutPaidUseCase{
		payoutGetter: payoutGetter, payouts: payouts, bookings: bookings, spots: spots, storage: storage, events: events,
	}
}

// Execute marks payoutID PAID on behalf of adminUserID. proof/proofContentType are the
// optional bank-transfer proof image bytes -- pass a nil/empty proof to mark paid without one.
func (uc *MarkPayoutPaidUseCase) Execute(ctx context.Context, payoutID, adminUserID string, proof []byte, proofContentType string) (entity.Payout, error) {
	var proofURL *string
	if len(proof) > 0 {
		// Needed before the payout is marked paid: the proof's storage key is
		// {ownerId}/{paymentId}, not {payoutId} -- see storage.Client.UploadPayoutProof.
		existing, err := uc.payoutGetter.GetPayoutByID(ctx, payoutID)
		if err != nil {
			return entity.Payout{}, err
		}
		url, err := uc.storage.UploadPayoutProof(ctx, existing.OwnerID, existing.PaymentID, proofContentType, proof)
		if err != nil {
			return entity.Payout{}, err
		}
		proofURL = &url
	}

	payout, err := uc.payouts.MarkPayoutPaid(ctx, payoutID, adminUserID, proofURL)
	if err != nil {
		return entity.Payout{}, err
	}

	uc.publishPayoutPaid(ctx, payout)

	return payout, nil
}

// publishPayoutPaid resolves the booking and spot needed to build the payout.paid
// notification and emits it. The payout has already been marked PAID by the time this runs
// -- that's the source of truth -- so a failure resolving either lookup, or to publish, is
// swallowed rather than surfaced: a missed notification degrades the owner's experience but
// must not undo, or appear to fail, a transfer that already went out.
func (uc *MarkPayoutPaidUseCase) publishPayoutPaid(ctx context.Context, payout entity.Payout) {
	booking, err := uc.bookings.GetByID(ctx, payout.BookingID)
	if err != nil {
		return
	}
	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return
	}
	emitBookingEvent(ctx, uc.events, RoutingKeyPayoutPaid, newPayoutPaidEvent(payout, spot.Title))
}
