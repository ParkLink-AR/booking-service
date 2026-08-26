package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

// GetBooking returns a single booking by id.
func (service *Service) GetBooking(ctx context.Context, bookingID string) (entity.Booking, error) {
	return service.bookingRepository.GetBookingByID(ctx, bookingID)
}
