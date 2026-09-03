package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_reviews_by_booking_repository_mock.go -package=mocks -source=./list_reviews_by_booking_usecase.go

// ReviewListerByBooking reads every review (by anyone, for any target) tied to one
// booking. Implemented by internal/repository/booking.
type ReviewListerByBooking interface {
	ListReviewsByBooking(ctx context.Context, bookingID string) ([]entity.Review, error)
}

// ListReviewsByBookingUseCase powers GET /bookings/{id}/reviews -- mobile uses it to
// figure out what's left to review on a completed booking.
type ListReviewsByBookingUseCase struct {
	lister ReviewListerByBooking
}

func NewListReviewsByBookingUseCase(lister ReviewListerByBooking) *ListReviewsByBookingUseCase {
	return &ListReviewsByBookingUseCase{lister: lister}
}

func (uc *ListReviewsByBookingUseCase) Execute(ctx context.Context, bookingID string) ([]entity.Review, error) {
	return uc.lister.ListReviewsByBooking(ctx, bookingID)
}
