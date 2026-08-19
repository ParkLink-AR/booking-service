package parkingservice

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/parking_service_repository_mock.go -package=mocks -source=./service.go

// ParkingServiceRepository fetches the published availability base for a parking spot
// from parking-service. Implemented by a fake in this iteration
// (internal/repository/parkingservice); a real HTTP-backed repository will replace it
// once parking-service exists.
type ParkingServiceRepository interface {
	GetAvailabilityBase(ctx context.Context, parkingSpotID string, from, to time.Time) (*entity.AvailabilityBase, error)
}

// Service is a pass-through to the parking-service repository. It exists purely so the
// usecase orchestrates through the service layer only, keeping the same five-layer chain
// (controller -> usecase -> service -> repository -> domain) that every feature in this
// project uses — the usecase never talks to a repository (or a "client") directly.
type Service struct {
	parkingServiceRepository ParkingServiceRepository
}

func NewService(parkingServiceRepository ParkingServiceRepository) *Service {
	return &Service{parkingServiceRepository: parkingServiceRepository}
}

func (service *Service) GetAvailabilityBase(
	ctx context.Context,
	parkingSpotID string,
	from, to time.Time,
) (*entity.AvailabilityBase, error) {
	return service.parkingServiceRepository.GetAvailabilityBase(ctx, parkingSpotID, from, to)
}
