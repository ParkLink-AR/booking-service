package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_owner_bookings_repository_mock.go -package=mocks -source=./list_owner_bookings_usecase.go

// OwnerSpotIDsLookup resolves every parking spot id owned by a user. Implemented by
// internal/repository/parkingclient.
type OwnerSpotIDsLookup interface {
	GetSpotIDsForOwner(ctx context.Context, ownerID string) ([]string, error)
}

// OwnerSpotBookingLister returns every booking for any of a set of parking spot ids.
// Implemented by internal/repository/booking.
type OwnerSpotBookingLister interface {
	ListBookingsByOwnerSpots(ctx context.Context, parkingSpotIDs []string) ([]entity.Booking, error)
}

// ListOwnerBookingsUseCase powers GET /admin/owners/{id}/bookings -- every reservation
// received across all of an owner's parking spots, for ParkLink staff to review.
type ListOwnerBookingsUseCase struct {
	spots    OwnerSpotIDsLookup
	bookings OwnerSpotBookingLister
}

func NewListOwnerBookingsUseCase(spots OwnerSpotIDsLookup, bookings OwnerSpotBookingLister) *ListOwnerBookingsUseCase {
	return &ListOwnerBookingsUseCase{spots: spots, bookings: bookings}
}

func (uc *ListOwnerBookingsUseCase) Execute(ctx context.Context, ownerID string) ([]entity.Booking, error) {
	spotIDs, err := uc.spots.GetSpotIDsForOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if len(spotIDs) == 0 {
		return []entity.Booking{}, nil
	}
	return uc.bookings.ListBookingsByOwnerSpots(ctx, spotIDs)
}
