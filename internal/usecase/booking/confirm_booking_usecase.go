package booking

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
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

// Execute confirms the booking identified by bookingID, attaching paymentID.
func (uc *ConfirmBookingUseCase) Execute(ctx context.Context, bookingID, paymentID string) (entity.Booking, error) {
	booking, err := uc.confirmer.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	spot, err := uc.spotLookup.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, err
	}

	deadline := time.Now().Add(ownerApprovalWindow())
	pending, err := uc.confirmer.ConfirmBooking(ctx, bookingID, paymentID, enum.BookingStatusPendingOwnerApproval, &deadline, nil)
	if err != nil {
		return entity.Booking{}, err
	}
	emitBookingEvent(ctx, uc.events, RoutingKeyBookingPendingApproval,
		newBookingEvent(RoutingKeyBookingPendingApproval, pending, spot))
	return pending, nil
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
