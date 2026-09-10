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
	ConfirmBooking(ctx context.Context, bookingID, paymentID string, status enum.BookingStatus, ownerResponseDeadline *time.Time) (entity.Booking, error)
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
}

// SpotLookup resolves the parking spot behind a booking: whether it auto-confirms, who
// owns it, and its title. The confirm flow needs the flag to branch on, and the owner id
// plus title to populate the domain event it publishes. Implemented by
// internal/repository/parkingclient.
type SpotLookup interface {
	GetSpotDetails(ctx context.Context, spotID string) (entity.SpotDetails, error)
}

// ConfirmBookingUseCase orchestrates the booking-confirmation step triggered by the
// payment webhook: it decides, based on the spot's instant-booking setting, whether the
// booking becomes CONFIRMED right away or PENDING_OWNER_APPROVAL (awaiting the owner), and
// publishes the matching domain event (booking.confirmed / booking.pending_approval).
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

	if spot.InstantBooking {
		confirmed, err := uc.confirmer.ConfirmBooking(ctx, bookingID, paymentID, enum.BookingStatusConfirmed, nil)
		if err != nil {
			return entity.Booking{}, err
		}
		emitBookingEvent(ctx, uc.events, RoutingKeyBookingConfirmed,
			newBookingEvent(RoutingKeyBookingConfirmed, confirmed, spot, true))
		return confirmed, nil
	}

	deadline := time.Now().Add(ownerApprovalWindow())
	pending, err := uc.confirmer.ConfirmBooking(ctx, bookingID, paymentID, enum.BookingStatusPendingOwnerApproval, &deadline)
	if err != nil {
		return entity.Booking{}, err
	}
	emitBookingEvent(ctx, uc.events, RoutingKeyBookingPendingApproval,
		newBookingEvent(RoutingKeyBookingPendingApproval, pending, spot, false))
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
