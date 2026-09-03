package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// Booking represents a reservation of a parking spot for a time range.
type Booking struct {
	ID            string
	ParkingSpotID string
	UserID        string
	Start         time.Time
	End           time.Time
	Status        enum.BookingStatus
	HoldExpiresAt *time.Time
	// Set only when Status is PENDING_OWNER_APPROVAL: how long the owner has
	// to approve/reject before the sweep auto-cancels and refunds it.
	OwnerResponseDeadline *time.Time
	Amount                int
	PaymentID             string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// CreateBookingInput carries the data needed to create a new booking, threaded from the
// controller through the usecase into the service that owns the creation business logic.
// ParkingSpotSchedule is the spot's published weekly schedule as the caller knows it --
// booking-service validates the requested range against it but never fetches it itself.
type CreateBookingInput struct {
	ParkingSpotID       string
	ParkingSpotSchedule []ScheduleEntry
	UserID              string
	Start               time.Time
	End                 time.Time
	Amount              int
}
