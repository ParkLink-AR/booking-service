package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/verification_review_repository_mock.go -package=mocks -source=./verification_review_usecase.go

// VerificationReviewLister lists every paid, completed booking whose arrival was never
// verified in person and that's still awaiting an admin decision. Implemented by
// internal/repository/booking.
type VerificationReviewLister interface {
	ListBookingsPendingVerificationReview(ctx context.Context) ([]entity.Booking, error)
}

// VerificationReviewResolver atomically moves a booking's verification review out of
// PENDING. Implemented by internal/repository/booking.
type VerificationReviewResolver interface {
	ResolveVerificationReview(ctx context.Context, bookingID string, review enum.VerificationReview) (entity.Booking, error)
}

// ListPendingVerificationReviewsUseCase backs the admin panel's "reservas sin verificar"
// queue.
type ListPendingVerificationReviewsUseCase struct {
	lister VerificationReviewLister
}

func NewListPendingVerificationReviewsUseCase(lister VerificationReviewLister) *ListPendingVerificationReviewsUseCase {
	return &ListPendingVerificationReviewsUseCase{lister: lister}
}

// Execute returns every booking whose verification review is still PENDING.
func (uc *ListPendingVerificationReviewsUseCase) Execute(ctx context.Context) ([]entity.Booking, error) {
	return uc.lister.ListBookingsPendingVerificationReview(ctx)
}

// ResolveVerificationReviewUseCase records an admin's decision on a booking that completed
// without its arrival ever being verified: APPROVED lets the owner's payout go ahead,
// REFUNDED gives the driver their money back and the owner is never paid for it.
type ResolveVerificationReviewUseCase struct {
	resolver VerificationReviewResolver
	refunder PaymentRefunder
}

func NewResolveVerificationReviewUseCase(resolver VerificationReviewResolver, refunder PaymentRefunder) *ResolveVerificationReviewUseCase {
	return &ResolveVerificationReviewUseCase{resolver: resolver, refunder: refunder}
}

// Execute resolves bookingID's PENDING review as review, which must be APPROVED or
// REFUNDED.
func (uc *ResolveVerificationReviewUseCase) Execute(
	ctx context.Context,
	bookingID string,
	review enum.VerificationReview,
) (entity.Booking, error) {
	if review != enum.VerificationReviewApproved && review != enum.VerificationReviewRefunded {
		return entity.Booking{}, errors.NewInvalidInput("INVALID_DECISION", "decision must be APPROVED or REFUNDED")
	}

	resolved, err := uc.resolver.ResolveVerificationReview(ctx, bookingID, review)
	if err != nil {
		return entity.Booking{}, err
	}

	if review == enum.VerificationReviewRefunded {
		// Marked REFUNDED first (so the payout sweep can never pick it up in between) --
		// a refund failure after that needs manual follow-up, surfaced to the admin as an
		// upstream error rather than silently swallowed, same as CancelBookingUseCase.
		if err := uc.refunder.RefundPayment(ctx, resolved.PaymentID); err != nil {
			return entity.Booking{}, err
		}
	}
	return resolved, nil
}
