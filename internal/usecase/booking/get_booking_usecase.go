package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_getter_service_mock.go -package=mocks -source=./get_booking_usecase.go

// BookingGetter returns a single booking by id. Implemented by internal/service/booking.
type BookingGetter interface {
	GetBooking(ctx context.Context, bookingID string) (entity.Booking, error)
}

// GetBookingUseCase orchestrates the get-a-booking flow. It contains no business logic of
// its own — it only ever talks to the booking service.
type GetBookingUseCase struct {
	bookingGetter BookingGetter
}

func NewGetBookingUseCase(bookingGetter BookingGetter) *GetBookingUseCase {
	return &GetBookingUseCase{bookingGetter: bookingGetter}
}

// Execute returns the booking identified by bookingID.
func (useCase *GetBookingUseCase) Execute(ctx context.Context, bookingID string) (entity.Booking, error) {
	return useCase.bookingGetter.GetBooking(ctx, bookingID)
}
