package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_reviews_by_target_repository_mock.go -package=mocks -source=./list_reviews_by_target_usecase.go

// ReviewListerByTarget reads every review left for a spot, an owner, or a driver.
// Implemented by internal/repository/booking.
type ReviewListerByTarget interface {
	ListReviewsByTarget(ctx context.Context, targetType enum.ReviewTargetType, targetID string) ([]entity.Review, error)
}

// ListReviewsByTargetUseCase powers both GET /parking-spots/{id}/reviews (targetType
// spot) and GET /users/{id}/reviews?as=owner|driver -- same shape either way.
type ListReviewsByTargetUseCase struct {
	lister ReviewListerByTarget
}

func NewListReviewsByTargetUseCase(lister ReviewListerByTarget) *ListReviewsByTargetUseCase {
	return &ListReviewsByTargetUseCase{lister: lister}
}

func (uc *ListReviewsByTargetUseCase) Execute(ctx context.Context, targetType enum.ReviewTargetType, targetID string) ([]entity.Review, error) {
	return uc.lister.ListReviewsByTarget(ctx, targetType, targetID)
}
