package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_history_service_mock.go -package=mocks -source=./list_booking_history_usecase.go

// BookingHistoryLister returns every booking made by a user, across all parking spots.
// Implemented by internal/service/booking.
type BookingHistoryLister interface {
	ListBookingsForUser(ctx context.Context, userID string) ([]entity.Booking, error)
}

// ListBookingHistoryUseCase orchestrates the booking-history flow. It contains no business
// logic of its own — it only ever talks to the booking service.
type ListBookingHistoryUseCase struct {
	bookingHistoryLister BookingHistoryLister
}

func NewListBookingHistoryUseCase(bookingHistoryLister BookingHistoryLister) *ListBookingHistoryUseCase {
	return &ListBookingHistoryUseCase{bookingHistoryLister: bookingHistoryLister}
}

// Execute returns every booking made by userID.
func (useCase *ListBookingHistoryUseCase) Execute(ctx context.Context, userID string) ([]entity.Booking, error) {
	return useCase.bookingHistoryLister.ListBookingsForUser(ctx, userID)
}
