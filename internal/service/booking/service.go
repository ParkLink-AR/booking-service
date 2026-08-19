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
}

// ParkingAvailability fetches the published availability base for a parking spot, used to
// confirm a requested booking falls within the spot's published hours.
type ParkingAvailability interface {
	GetAvailabilityBase(ctx context.Context, parkingSpotID string, from, to time.Time) (*entity.AvailabilityBase, error)
}

// Service owns the business logic for a parking spot's bookings: listing them grouped by
// day, and creating new ones (validating the range, published hours, and overlap with
// existing active bookings).
type Service struct {
	bookingRepository   BookingRepository
	parkingAvailability ParkingAvailability
	location            *time.Location
	holdDuration        time.Duration
}

func NewService(
	bookingRepository BookingRepository,
	parkingAvailability ParkingAvailability,
	location *time.Location,
	holdDuration time.Duration,
) *Service {
	return &Service{
		bookingRepository:   bookingRepository,
		parkingAvailability: parkingAvailability,
		location:            location,
		holdDuration:        holdDuration,
	}
}
