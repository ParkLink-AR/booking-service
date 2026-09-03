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

// SpotLookup answers whether a parking spot auto-confirms bookings. Implemented by
// internal/repository/parkingclient.
type SpotLookup interface {
	IsInstantBooking(ctx context.Context, spotID string) (bool, error)
}

// ConfirmBookingUseCase orchestrates the booking-confirmation step triggered by the
// payment webhook: it decides, based on the spot's instant-booking setting, whether the
// booking becomes CONFIRMED right away or PENDING_OWNER_APPROVAL (awaiting the owner).
type ConfirmBookingUseCase struct {
	confirmer  BookingConfirmer
	spotLookup SpotLookup
}

func NewConfirmBookingUseCase(confirmer BookingConfirmer, spotLookup SpotLookup) *ConfirmBookingUseCase {
	return &ConfirmBookingUseCase{confirmer: confirmer, spotLookup: spotLookup}
}

// Execute confirms the booking identified by bookingID, attaching paymentID.
func (uc *ConfirmBookingUseCase) Execute(ctx context.Context, bookingID, paymentID string) (entity.Booking, error) {
	booking, err := uc.confirmer.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	instantBooking, err := uc.spotLookup.IsInstantBooking(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, err
	}

	if instantBooking {
		return uc.confirmer.ConfirmBooking(ctx, bookingID, paymentID, enum.BookingStatusConfirmed, nil)
	}

	deadline := time.Now().Add(ownerApprovalWindow())
	return uc.confirmer.ConfirmBooking(ctx, bookingID, paymentID, enum.BookingStatusPendingOwnerApproval, &deadline)
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
