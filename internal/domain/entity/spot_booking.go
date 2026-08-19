package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// SpotBooking is a booking's status and hold, scoped to one parking spot, as returned when
// checking bookings across many spots at once. It carries only what's needed to decide
// whether the booking currently occupies the spot — the caller applies that business rule.
type SpotBooking struct {
	ParkingSpotID string
	Status        enum.BookingStatus
	HoldExpiresAt *time.Time
}
