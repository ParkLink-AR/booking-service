package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

// BookingsByUserLister lists all bookings for a given user. Implemented by the repository.
type BookingsByUserLister interface {
	ListBookingsByUser(
		ctx context.Context,
		userID string,
		order enum.UserBookingsOrder,
		status *enum.BookingStatus,
	) ([]entity.Booking, error)
}

// ListBookingsByUserUseCase returns a given user's bookings, optionally only those in one status,
// in the requested order.
type ListBookingsByUserUseCase struct {
	lister BookingsByUserLister
}

func NewListBookingsByUserUseCase(lister BookingsByUserLister) *ListBookingsByUserUseCase {
	return &ListBookingsByUserUseCase{lister: lister}
}

func (uc *ListBookingsByUserUseCase) Execute(
	ctx context.Context,
	userID string,
	order enum.UserBookingsOrder,
	status *enum.BookingStatus,
) ([]entity.Booking, error) {
	return uc.lister.ListBookingsByUser(ctx, userID, order, status)
}
