package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_confirmation_repository_mock.go -package=mocks -source=./confirm_booking_usecase.go

// BookingConfirmer atomically confirms a pending booking by setting its payment_id and
// transitioning its status to CONFIRMED. Implemented by internal/repository/booking.
type BookingConfirmer interface {
	ConfirmBooking(ctx context.Context, bookingID, paymentID string) (entity.Booking, error)
}

// ConfirmBookingUseCase orchestrates the booking-confirmation step triggered by the
// payment webhook: it has no business logic of its own beyond delegating to the
// repository's atomic UPDATE.
type ConfirmBookingUseCase struct {
	confirmer BookingConfirmer
}

func NewConfirmBookingUseCase(confirmer BookingConfirmer) *ConfirmBookingUseCase {
	return &ConfirmBookingUseCase{confirmer: confirmer}
}

// Execute confirms the booking identified by bookingID, attaching paymentID.
func (uc *ConfirmBookingUseCase) Execute(ctx context.Context, bookingID, paymentID string) (entity.Booking, error) {
	return uc.confirmer.ConfirmBooking(ctx, bookingID, paymentID)
}
