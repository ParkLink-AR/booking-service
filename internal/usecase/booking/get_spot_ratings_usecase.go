package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_spot_ratings_repository_mock.go -package=mocks -source=./get_spot_ratings_usecase.go

// SpotRatingsGetter reads the average rating and review count for a batch of parking
// spots in one call. Implemented by internal/repository/booking.
type SpotRatingsGetter interface {
	GetSpotRatings(ctx context.Context, spotIDs []string) (map[string]entity.RatingSummary, error)
}

// GetSpotRatingsUseCase powers the internal, service-to-service batch endpoint
// parking-service calls to embed real ratings in its own search/detail responses --
// mirrors GetBookableSpotsUseCase's role for availability.
type GetSpotRatingsUseCase struct {
	ratings SpotRatingsGetter
}

func NewGetSpotRatingsUseCase(ratings SpotRatingsGetter) *GetSpotRatingsUseCase {
	return &GetSpotRatingsUseCase{ratings: ratings}
}

func (uc *GetSpotRatingsUseCase) Execute(ctx context.Context, spotIDs []string) (map[string]entity.RatingSummary, error) {
	return uc.ratings.GetSpotRatings(ctx, spotIDs)
}
