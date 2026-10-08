package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/complete_finished_bookings_repository_mock.go -package=mocks -source=./complete_finished_bookings_usecase.go

// FinishedBookingCompleter marks every booking whose time window has ended COMPLETED and
// returns them. Implemented by internal/repository/booking.
type FinishedBookingCompleter interface {
	ExpireCompletedBookings(ctx context.Context) ([]entity.Booking, error)
}

// CompleteFinishedBookingsUseCase closes every stay whose time window is over and
// publishes booking.completed for each one, so both sides are asked to review the other.
// Meant to run on a ticker (see cmd.runCompletionSweepJob).
type CompleteFinishedBookingsUseCase struct {
	bookings FinishedBookingCompleter
	spots    SpotDetailsLookup
	events   EventPublisher
}

func NewCompleteFinishedBookingsUseCase(bookings FinishedBookingCompleter, spots SpotDetailsLookup, events EventPublisher) *CompleteFinishedBookingsUseCase {
	return &CompleteFinishedBookingsUseCase{bookings: bookings, spots: spots, events: events}
}

// Execute returns how many bookings it completed, so the caller can log it. The status
// change is already committed when the events go out; a booking whose spot can't be
// looked up is still announced, without the owner (only the driver gets asked).
func (uc *CompleteFinishedBookingsUseCase) Execute(ctx context.Context) (int, error) {
	completed, err := uc.bookings.ExpireCompletedBookings(ctx)
	if err != nil {
		return 0, err
	}
	for _, b := range completed {
		spot, _ := uc.spots.GetSpotDetails(ctx, b.ParkingSpotID)
		emitBookingEvent(ctx, uc.events, RoutingKeyBookingCompleted,
			newBookingEvent(RoutingKeyBookingCompleted, b, spot))
	}
	return len(completed), nil
}
