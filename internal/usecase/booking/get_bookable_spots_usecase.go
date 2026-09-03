package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/bookable_spots_service_mock.go -package=mocks -source=./get_bookable_spots_usecase.go

// BookableSpotsFinder returns the subset of spot IDs that still have room (fewer active
// bookings than their capacity) in a given time range. Implemented by
// internal/service/booking.
type BookableSpotsFinder interface {
	GetBookableSpots(ctx context.Context, spots []entity.SpotCapacity, start, end time.Time) ([]string, error)
}

// GetBookableSpotsUseCase orchestrates the bookable-spots flow. It contains no business
// logic of its own — it only ever talks to the booking service.
type GetBookableSpotsUseCase struct {
	bookableSpotsFinder BookableSpotsFinder
}

func NewGetBookableSpotsUseCase(bookableSpotsFinder BookableSpotsFinder) *GetBookableSpotsUseCase {
	return &GetBookableSpotsUseCase{bookableSpotsFinder: bookableSpotsFinder}
}

// Execute returns the subset of spot IDs that still have room in [start, end).
func (useCase *GetBookableSpotsUseCase) Execute(
	ctx context.Context,
	spots []entity.SpotCapacity,
	start, end time.Time,
) ([]string, error) {
	return useCase.bookableSpotsFinder.GetBookableSpots(ctx, spots, start, end)
}
