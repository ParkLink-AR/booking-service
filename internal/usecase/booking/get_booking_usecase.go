package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_booking_repository_mock.go -package=mocks -source=./get_booking_usecase.go

// BookingGetter reads a single booking by id. Implemented by internal/repository/booking.
type BookingGetter interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
}

// GetBookingUseCase powers GET /bookings/{id} -- the driver polls this after paying to
// find out whether their booking confirmed outright or is waiting on the owner.
type GetBookingUseCase struct {
	getter BookingGetter
}

func NewGetBookingUseCase(getter BookingGetter) *GetBookingUseCase {
	return &GetBookingUseCase{getter: getter}
}

func (uc *GetBookingUseCase) Execute(ctx context.Context, bookingID string) (entity.Booking, error) {
	return uc.getter.GetByID(ctx, bookingID)
}
