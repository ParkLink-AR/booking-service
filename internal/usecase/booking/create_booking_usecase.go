package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_creation_service_mock.go -package=mocks -source=./create_booking_usecase.go

// BookingCreator creates a new booking, including all its business validation (range,
// published hours, overlap with existing bookings). Implemented by internal/service/booking.
type BookingCreator interface {
	CreateBooking(ctx context.Context, input entity.CreateBookingInput) (entity.Booking, error)
}

// CreateBookingUseCase orchestrates the booking-creation flow. It contains no business
// logic of its own — it only ever talks to the booking service.
type CreateBookingUseCase struct {
	bookingCreator BookingCreator
}

func NewCreateBookingUseCase(bookingCreator BookingCreator) *CreateBookingUseCase {
	return &CreateBookingUseCase{bookingCreator: bookingCreator}
}

// Execute creates a new booking from input.
func (useCase *CreateBookingUseCase) Execute(ctx context.Context, input entity.CreateBookingInput) (entity.Booking, error) {
	return useCase.bookingCreator.CreateBooking(ctx, input)
}
