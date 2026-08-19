package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_listing_service_mock.go -package=mocks -source=./list_bookings_usecase.go

// BookingLister lists a parking spot's bookings grouped by day, including validating the
// requested range. Implemented by internal/service/booking.
type BookingLister interface {
	ListBookings(
		ctx context.Context,
		parkingSpotID string,
		from, to *time.Time,
		status *enum.BookingStatus,
	) ([]entity.DayBookings, error)
}

// ListBookingsUseCase orchestrates the booking-listing flow. It contains no business logic
// of its own — it only ever talks to the booking service.
type ListBookingsUseCase struct {
	bookingLister BookingLister
}

func NewListBookingsUseCase(bookingLister BookingLister) *ListBookingsUseCase {
	return &ListBookingsUseCase{bookingLister: bookingLister}
}

// Execute returns the parking spot's bookings grouped by day.
func (useCase *ListBookingsUseCase) Execute(
	ctx context.Context,
	parkingSpotID string,
	from, to *time.Time,
	status *enum.BookingStatus,
) ([]entity.DayBookings, error) {
	return useCase.bookingLister.ListBookings(ctx, parkingSpotID, from, to, status)
}
