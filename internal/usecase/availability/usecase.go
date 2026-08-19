package availability

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/availability_usecase_dependencies_mock.go -package=mocks -source=./usecase.go

// ParkingService fetches the published availability base for a parking spot.
// Implemented by internal/service/parkingservice.
type ParkingService interface {
	GetAvailabilityBase(ctx context.Context, parkingSpotID string, from, to time.Time) (*entity.AvailabilityBase, error)
}

// AvailabilityService executes all the business logic for turning a published availability
// base into effective per-day availability: it validates the requested range, loads active
// bookings, and subtracts them from the base. Implemented by internal/service/availability.
type AvailabilityService interface {
	GetAvailability(
		ctx context.Context,
		parkingSpotID string,
		from, to time.Time,
		availabilityBase *entity.AvailabilityBase,
	) ([]entity.DayAvailability, error)
}

// GetAvailabilityUseCase orchestrates the availability flow: it fetches the published
// availability base from the parking service, then delegates to the availability service
// for everything else. It contains no business logic of its own, and it only ever talks
// to services — never directly to a repository or an external client.
type GetAvailabilityUseCase struct {
	parkingService      ParkingService
	availabilityService AvailabilityService
}

func NewGetAvailabilityUseCase(
	parkingService ParkingService,
	availabilityService AvailabilityService,
) *GetAvailabilityUseCase {
	return &GetAvailabilityUseCase{
		parkingService:      parkingService,
		availabilityService: availabilityService,
	}
}

// Execute returns effective availability for each day in the half-open range [from, to).
func (useCase *GetAvailabilityUseCase) Execute(
	ctx context.Context,
	parkingSpotID string,
	from, to time.Time,
) ([]entity.DayAvailability, error) {
	availabilityBase, err := useCase.parkingService.GetAvailabilityBase(ctx, parkingSpotID, from, to)
	if err != nil {
		return nil, errors.NewUpstream("availability.GetAvailabilityBase", err)
	}

	return useCase.availabilityService.GetAvailability(ctx, parkingSpotID, from, to, availabilityBase)
}
