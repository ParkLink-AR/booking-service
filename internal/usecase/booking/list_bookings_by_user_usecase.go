package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

// BookingsByUserLister lists all bookings for a given user. Implemented by the repository.
type BookingsByUserLister interface {
	ListBookingsByUser(ctx context.Context, userID string) ([]entity.Booking, error)
}

// ListBookingsByUserUseCase returns all bookings for a given user.
type ListBookingsByUserUseCase struct {
	lister BookingsByUserLister
}

func NewListBookingsByUserUseCase(lister BookingsByUserLister) *ListBookingsByUserUseCase {
	return &ListBookingsByUserUseCase{lister: lister}
}

func (uc *ListBookingsByUserUseCase) Execute(ctx context.Context, userID string) ([]entity.Booking, error) {
	return uc.lister.ListBookingsByUser(ctx, userID)
}
