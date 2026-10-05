package booking

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_confirmation_repository_mock.go -package=mocks -source=./confirm_booking_usecase.go

// BookingConfirmer atomically confirms a pending booking by setting its payment_id and
// transitioning its status to CONFIRMED or PENDING_OWNER_APPROVAL. Implemented by
// internal/repository/booking.
type BookingConfirmer interface {
	ConfirmBooking(ctx context.Context, bookingID, paymentID string, status enum.BookingStatus, ownerResponseDeadline *time.Time, verificationCode *string) (entity.Booking, error)
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
}

// SpotLookup resolves the parking spot behind a booking: who owns it and its title, to
// populate the domain event the confirm flow publishes. Implemented by
// internal/repository/parkingclient.
type SpotLookup interface {
	GetSpotDetails(ctx context.Context, spotID string) (entity.SpotDetails, error)
}

// ConfirmBookingUseCase orchestrates the booking-confirmation step triggered by the
// payment webhook: every paid booking moves to PENDING_OWNER_APPROVAL (awaiting the
// owner), and it publishes the matching domain event (booking.pending_approval).
type ConfirmBookingUseCase struct {
	confirmer  BookingConfirmer
	spotLookup SpotLookup
	events     EventPublisher
}

func NewConfirmBookingUseCase(confirmer BookingConfirmer, spotLookup SpotLookup, events EventPublisher) *ConfirmBookingUseCase {
	return &ConfirmBookingUseCase{confirmer: confirmer, spotLookup: spotLookup, events: events}
}

// Error codes ConfirmBookingUseCase returns for a payment that must NOT confirm the
// booking. payment-service refunds the payment on any of them -- the money moved, but
// there's no booking it can pay for.
const (
	// ConfirmErrorHoldExpired: the payment arrived after the booking's hold elapsed (e.g. an
	// offline payment approved hours later), so the slot may already be someone else's.
	ConfirmErrorHoldExpired = "HOLD_EXPIRED"
	// ConfirmErrorAmountMismatch: the amount actually charged isn't the booking's price.
	ConfirmErrorAmountMismatch = "AMOUNT_MISMATCH"
	// ConfirmErrorNotPayable: the booking was already paid with a different payment, or
	// isn't awaiting payment at all (e.g. cancelled).
	ConfirmErrorNotPayable = "BOOKING_NOT_PAYABLE"
)

// Execute confirms the booking identified by bookingID with the approved payment
// paymentID, which charged amount (centavos). It's idempotent for the same paymentID --
// the webhook and the app's polling can both report the same approval -- and rejects,
// with one of the ConfirmError* codes, a payment that came too late, charged the wrong
// amount, or targets a booking that isn't awaiting payment.
func (uc *ConfirmBookingUseCase) Execute(ctx context.Context, bookingID, paymentID string, amount int) (entity.Booking, error) {
	booking, err := uc.confirmer.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}
	if booking.PaymentID == paymentID && booking.PaidAt != nil {
		return booking, nil
	}
	if err := payableError(booking, amount); err != nil {
		return entity.Booking{}, err
	}

	spot, err := uc.spotLookup.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, err
	}

	deadline := time.Now().Add(ownerApprovalWindow())
	pending, err := uc.confirmer.ConfirmBooking(ctx, bookingID, paymentID, enum.BookingStatusPendingOwnerApproval, &deadline, nil)
	if err != nil {
		if !errors.IsNotFound(err) {
			return entity.Booking{}, err
		}
		// Lost a race: either the same payment confirmed it a moment ago (fine), or the
		// hold expired between the check above and the conditional UPDATE.
		current, getErr := uc.confirmer.GetByID(ctx, bookingID)
		if getErr == nil && current.PaymentID == paymentID && current.PaidAt != nil {
			return current, nil
		}
		return entity.Booking{}, errors.NewInvalidInput(ConfirmErrorHoldExpired, "la reserva venció antes de que se acreditara el pago")
	}
	emitBookingEvent(ctx, uc.events, RoutingKeyBookingPendingApproval,
		newBookingEvent(RoutingKeyBookingPendingApproval, pending, spot))
	return pending, nil
}

// payableError reports why booking can't be confirmed with a payment of amount, or nil
// if it can.
func payableError(booking entity.Booking, amount int) error {
	holdExpired := booking.HoldExpiresAt != nil && !time.Now().Before(*booking.HoldExpiresAt)
	switch {
	case booking.Status == enum.BookingStatusExpired,
		booking.Status == enum.BookingStatusPendingPayment && holdExpired:
		return errors.NewInvalidInput(ConfirmErrorHoldExpired, "la reserva venció antes de que se acreditara el pago")
	case booking.Status != enum.BookingStatusPendingPayment:
		return errors.NewInvalidInput(ConfirmErrorNotPayable, "la reserva no está esperando un pago")
	case amount != booking.Amount:
		return errors.NewInvalidInput(ConfirmErrorAmountMismatch, "el monto pagado no coincide con el de la reserva")
	}
	return nil
}

// ownerApprovalWindow reads OWNER_APPROVAL_HOURS, falling back to 24h when unset or not a
// valid positive integer.
func ownerApprovalWindow() time.Duration {
	hours := 24
	if raw := os.Getenv("OWNER_APPROVAL_HOURS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			hours = value
		}
	}
	return time.Duration(hours) * time.Hour
}
