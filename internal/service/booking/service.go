package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_listing_repository_mock.go -package=mocks -source=./service.go

// BookingRepository loads and creates bookings for a parking spot.
type BookingRepository interface {
	ListBookings(
		ctx context.Context,
		parkingSpotID string,
		from, to *time.Time,
		status *enum.BookingStatus,
	) ([]entity.BookingWindow, error)
	GetActiveBookingsInRange(ctx context.Context, parkingSpotID string, from, to time.Time) ([]entity.TimeWindow, error)
	CreateBooking(ctx context.Context, booking entity.Booking) (entity.Booking, error)
	GetBookingsForSpotsInRange(ctx context.Context, parkingSpotIDs []string, start, end time.Time) ([]entity.SpotBooking, error)
	GetBookingByID(ctx context.Context, bookingID string) (entity.Booking, error)
	ListBookingsForUser(ctx context.Context, userID string) ([]entity.Booking, error)
}

// Service owns the business logic for a parking spot's bookings: listing them grouped by
// day, and creating new ones (validating the range, published hours -- against the
// schedule the caller provides, not fetched from anywhere -- and overlap with existing
// active bookings).
type Service struct {
	bookingRepository BookingRepository
	location          *time.Location
	holdDuration      time.Duration
}

func NewService(
	bookingRepository BookingRepository,
	location *time.Location,
	holdDuration time.Duration,
) *Service {
	return &Service{
		bookingRepository: bookingRepository,
		location:          location,
		holdDuration:      holdDuration,
	}
}
