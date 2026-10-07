package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/account_closure_repository_mock.go -package=mocks -source=./account_closure_usecase.go

// AccountClosureRepository is what AccountClosureUseCase needs from storage.
// Implemented by internal/repository/booking.
type AccountClosureRepository interface {
	ListAccountBookings(ctx context.Context, userID string, spotIDs []string) ([]entity.Booking, error)
	CancelBookingWithReason(ctx context.Context, bookingID, reason string) (entity.Booking, error)
	ExpireHold(ctx context.Context, bookingID string) error
	GetOwnerPendingPayouts(ctx context.Context, ownerID string, spotIDs []string) (int, int, error)
	ClearVehicleSnapshot(ctx context.Context, userID string) error
}

// Who cancelled: the side whose account was closed. Carried on the booking.cancelled event
// so notifications-service tells the other side.
const (
	cancelledByOwnerAccount  = "owner_account"
	cancelledByDriverAccount = "driver_account"
)

// AccountClosureUseCase is the bookings side of closing or suspending an account (called
// by auth-service, see app/services/account_lifecycle.py there). The account is userID
// as a driver, and spotIDs are the spots it owns.
//
//   - a booking in progress (the car is parked right now) blocks the closure;
//   - every booking that hasn't started is cancelled with a full refund, whichever side
//     the account is on, and the other side is notified;
//   - payment holds still waiting for the driver are dropped.
type AccountClosureUseCase struct {
	repository AccountClosureRepository
	refunder   PaymentRefunder
	spots      SpotDetailsLookup
	events     EventPublisher
	now        func() time.Time
}

func NewAccountClosureUseCase(
	repository AccountClosureRepository, refunder PaymentRefunder, spots SpotDetailsLookup, events EventPublisher,
) *AccountClosureUseCase {
	return &AccountClosureUseCase{repository: repository, refunder: refunder, spots: spots, events: events, now: time.Now}
}

type closureAction int

const (
	closureNone closureAction = iota
	closureInProgress
	closureExpireHold
	closureCancelAsDriver
	closureCancelAsOwner
	closureRejectAsOwner
)

func classifyForClosure(booking entity.Booking, userID string, now time.Time) closureAction {
	asDriver := booking.UserID == userID
	switch booking.Status {
	case enum.BookingStatusInProgress:
		return closureInProgress
	case enum.BookingStatusPendingPayment:
		if asDriver {
			return closureExpireHold
		}
		return closureNone
	case enum.BookingStatusConfirmed:
		// Already started: the driver may be parked even if the owner never entered the
		// arrival code. Ended ones are left to the completion sweep.
		if !now.Before(booking.Start) {
			if now.Before(booking.End) {
				return closureInProgress
			}
			return closureNone
		}
		if asDriver {
			return closureCancelAsDriver
		}
		return closureCancelAsOwner
	case enum.BookingStatusPendingOwnerApproval:
		if asDriver {
			return closureCancelAsDriver
		}
		return closureRejectAsOwner
	default:
		return closureNone
	}
}

// Summary reports what closing the account would do, without changing anything.
func (uc *AccountClosureUseCase) Summary(ctx context.Context, userID string, spotIDs []string) (entity.AccountClosureSummary, error) {
	bookings, err := uc.repository.ListAccountBookings(ctx, userID, spotIDs)
	if err != nil {
		return entity.AccountClosureSummary{}, err
	}

	var summary entity.AccountClosureSummary
	now := uc.now()
	for _, booking := range bookings {
		switch classifyForClosure(booking, userID, now) {
		case closureInProgress:
			summary.InProgressCount++
		case closureCancelAsDriver:
			summary.DriverBookingsToCancel++
		case closureCancelAsOwner:
			summary.OwnerBookingsToCancel++
		case closureRejectAsOwner:
			summary.OwnerRequestsToReject++
		}
	}

	summary.PendingPayoutCount, summary.PendingPayoutAmount, err = uc.repository.GetOwnerPendingPayouts(ctx, userID, spotIDs)
	if err != nil {
		return entity.AccountClosureSummary{}, err
	}
	return summary, nil
}

// CancelBookings cancels (with a full refund) every booking of the account that hasn't
// started, and drops its pending payment holds. Refused while a booking is in progress.
// Idempotent: what's already cancelled isn't listed again. A failed refund doesn't stop
// the rest -- the booking stays cancelled and is reported back for manual follow-up.
func (uc *AccountClosureUseCase) CancelBookings(ctx context.Context, userID string, spotIDs []string) (entity.AccountClosureResult, error) {
	bookings, err := uc.repository.ListAccountBookings(ctx, userID, spotIDs)
	if err != nil {
		return entity.AccountClosureResult{}, err
	}

	now := uc.now()
	for _, booking := range bookings {
		if classifyForClosure(booking, userID, now) == closureInProgress {
			return entity.AccountClosureResult{}, errors.NewConflict("la cuenta tiene una reserva en curso")
		}
	}

	var result entity.AccountClosureResult
	for _, booking := range bookings {
		action := classifyForClosure(booking, userID, now)
		switch action {
		case closureExpireHold:
			if err := uc.repository.ExpireHold(ctx, booking.ID); err != nil {
				return result, err
			}
			result.ExpiredHolds++
		case closureCancelAsDriver, closureCancelAsOwner, closureRejectAsOwner:
			reason, cancelledBy := constants.CancellationReasonOwnerAccountClosed, cancelledByOwnerAccount
			if action == closureCancelAsDriver {
				reason, cancelledBy = constants.CancellationReasonDriverAccountClosed, cancelledByDriverAccount
			}
			cancelled, err := uc.repository.CancelBookingWithReason(ctx, booking.ID, reason)
			if err != nil {
				if errors.IsNotFound(err) {
					continue // it changed state meanwhile (e.g. the driver cancelled it)
				}
				return result, err
			}
			result.Cancelled++
			if cancelled.PaymentID != "" {
				if err := uc.refunder.RefundPayment(ctx, cancelled.PaymentID); err != nil {
					result.RefundFailures = append(result.RefundFailures, cancelled.ID)
				}
			}
			uc.publishCancelled(ctx, cancelled, cancelledBy)
		}
	}
	return result, nil
}

type bookingCancelledEvent struct {
	bookingEvent
	CancelledBy string `json:"cancelled_by"`
}

func (uc *AccountClosureUseCase) publishCancelled(ctx context.Context, booking entity.Booking, cancelledBy string) {
	// The spot title only makes the notification nicer -- a failed lookup doesn't skip it.
	spot, _ := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	_ = uc.events.Publish(ctx, RoutingKeyBookingCancelled, bookingCancelledEvent{
		bookingEvent: newBookingEvent(RoutingKeyBookingCancelled, booking, spot),
		CancelledBy:  cancelledBy,
	})
}

// Anonymize wipes the personal data copied onto the driver's bookings (the vehicle
// snapshot) when their account's data is deleted. The bookings themselves stay.
func (uc *AccountClosureUseCase) Anonymize(ctx context.Context, userID string) error {
	return uc.repository.ClearVehicleSnapshot(ctx, userID)
}
