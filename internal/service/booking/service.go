package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/repository/parkingclient"
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
}

// ParkingClient is the subset of parkingclient.Client this service needs: pricing and
// validating a "stay" booking against the spot's owner-configured stay terms, and checking
// whether a spot accepts new reservations at all.
type ParkingClient interface {
	GetStayQuote(ctx context.Context, spotID, startDate, endDate string, startHour int) (parkingclient.StayQuote, error)
	IsSpotActive(ctx context.Context, spotID string) (bool, error)
}

// Service owns the business logic for a parking spot's bookings: listing them grouped by
// day, and creating new ones (validating the range, published hours -- against the
// schedule the caller provides, not fetched from anywhere -- and overlap with existing
// active bookings).
type Service struct {
	bookingRepository BookingRepository
	parkingClient     ParkingClient
	location          *time.Location
	holdDuration      time.Duration
}

func NewService(
	bookingRepository BookingRepository,
	parkingClient ParkingClient,
	location *time.Location,
	holdDuration time.Duration,
) *Service {
	return &Service{
		bookingRepository: bookingRepository,
		parkingClient:     parkingClient,
		location:          location,
		holdDuration:      holdDuration,
	}
}
