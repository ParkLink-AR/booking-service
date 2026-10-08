package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/expire_pending_approvals_repository_mock.go -package=mocks -source=./expire_pending_approvals_usecase.go

// StalePendingApprovalExpirer cancels every PENDING_OWNER_APPROVAL booking whose owner
// didn't answer in time and returns them. Implemented by internal/repository/booking.
type StalePendingApprovalExpirer interface {
	ExpireStalePendingApprovals(ctx context.Context) ([]entity.Booking, error)
}

// ExpirePendingApprovalsResult is what one run of ExpirePendingApprovalsUseCase did.
type ExpirePendingApprovalsResult struct {
	Expired int
	// Bookings that are cancelled but whose refund failed -- they need a manual follow-up.
	RefundFailures []string
}

// ExpirePendingApprovalsUseCase auto-cancels the booking requests their owner never
// answered within OWNER_APPROVAL_HOURS, refunds each one, and publishes booking.cancelled
// so the driver is told. Meant to run on a ticker (see cmd.runOwnerApprovalExpiryJob).
type ExpirePendingApprovalsUseCase struct {
	bookings StalePendingApprovalExpirer
	refunder PaymentRefunder
	spots    SpotDetailsLookup
	events   EventPublisher
}

func NewExpirePendingApprovalsUseCase(bookings StalePendingApprovalExpirer, refunder PaymentRefunder, spots SpotDetailsLookup, events EventPublisher) *ExpirePendingApprovalsUseCase {
	return &ExpirePendingApprovalsUseCase{bookings: bookings, refunder: refunder, spots: spots, events: events}
}

// Execute never stops at one booking's failure: each booking is already cancelled, so the
// rest still get their refund and their notice.
func (uc *ExpirePendingApprovalsUseCase) Execute(ctx context.Context) (ExpirePendingApprovalsResult, error) {
	expired, err := uc.bookings.ExpireStalePendingApprovals(ctx)
	if err != nil {
		return ExpirePendingApprovalsResult{}, err
	}
	result := ExpirePendingApprovalsResult{Expired: len(expired)}
	for _, b := range expired {
		if err := uc.refunder.RefundPayment(ctx, b.PaymentID); err != nil {
			result.RefundFailures = append(result.RefundFailures, b.ID)
		}
		// The spot title only makes the notification nicer -- a failed lookup doesn't skip it.
		spot, _ := uc.spots.GetSpotDetails(ctx, b.ParkingSpotID)
		emitBookingEvent(ctx, uc.events, RoutingKeyBookingCancelled,
			newBookingCancelledEvent(b, spot, CancelledByOwnerExpired))
	}
	return result, nil
}
