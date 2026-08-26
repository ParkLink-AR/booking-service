package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

// ListBookingsForUser returns every booking made by userID, across all parking spots.
func (service *Service) ListBookingsForUser(ctx context.Context, userID string) ([]entity.Booking, error) {
	return service.bookingRepository.ListBookingsForUser(ctx, userID)
}
