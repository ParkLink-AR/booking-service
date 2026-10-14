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
	GetActiveBookingsInRangeExcluding(ctx context.Context, parkingSpotID string, from, to time.Time, excludedBookingID string) ([]entity.TimeWindow, error)
	CreateBooking(ctx context.Context, booking entity.Booking) (entity.Booking, error)
	GetBookingsForSpotsInRange(ctx context.Context, parkingSpotIDs []string, start, end time.Time) ([]entity.SpotBooking, error)
}

// ParkingClient is the subset of parkingclient.Client this service needs: pricing and
// validating a "stay" booking against the spot's owner-configured stay terms, the owner's
// hourly rate for an "hourly" booking, checking whether a spot accepts new reservations
// at all, and who owns it (an owner can't book their own spot).
type ParkingClient interface {
	GetSpotOwnerID(ctx context.Context, spotID string) (string, error)
	GetStayQuote(ctx context.Context, spotID, startDate, endDate string, startHour int, vehicleType string) (parkingclient.StayQuote, error)
	GetHourlyPrice(ctx context.Context, spotID, vehicleType string) (float64, error)
	IsSpotActive(ctx context.Context, spotID string) (bool, error)
	GetVehicleRules(ctx context.Context, spotID string) (entity.SpotVehicleRules, error)
	GetBlockedDates(ctx context.Context, spotID string) ([]entity.DateRange, error)
	GetSpotBookingRules(ctx context.Context, spotID string) (entity.SpotBookingRules, error)
}

// CommissionRateReader reads the platform's current commission settings -- same shape as
// usecase.CommissionRateGetter, declared separately here per this codebase's
// consumer-defines-the-interface convention (service must not import the usecase package).
// Implemented by internal/repository/booking.
type CommissionRateReader interface {
	GetCommissionRate(ctx context.Context) (entity.PlatformSettings, error)
}

// Service owns the business logic for a parking spot's bookings: listing them grouped by
// day, and creating new ones (validating the range, published hours -- against the
// schedule the caller provides, not fetched from anywhere -- and overlap with existing
// active bookings).
type Service struct {
	bookingRepository BookingRepository
	parkingClient     ParkingClient
	commissionRates   CommissionRateReader
	location          *time.Location
	holdDuration      time.Duration
}

func NewService(
	bookingRepository BookingRepository,
	parkingClient ParkingClient,
	commissionRates CommissionRateReader,
	location *time.Location,
	holdDuration time.Duration,
) *Service {
	return &Service{
		bookingRepository: bookingRepository,
		parkingClient:     parkingClient,
		commissionRates:   commissionRates,
		location:          location,
		holdDuration:      holdDuration,
	}
}
