package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/submit_review_repository_mock.go -package=mocks -source=./submit_review_usecase.go

// ReviewCreator inserts a review, reporting whether it was actually a new row (false
// means the reviewer already rated this target for this booking). Implemented by
// internal/repository/booking.
type ReviewCreator interface {
	CreateReview(ctx context.Context, review entity.Review) (bool, entity.Review, error)
}

// SubmitReviewUseCase lets either side of a completed booking rate the other: the
// driver rates the spot and/or its owner, the owner rates the driver.
type SubmitReviewUseCase struct {
	getter  BookingGetter
	owners  SpotOwnerLookup
	reviews ReviewCreator
	spots   SpotDetailsLookup
	events  EventPublisher
}

func NewSubmitReviewUseCase(
	getter BookingGetter,
	owners SpotOwnerLookup,
	reviews ReviewCreator,
	spots SpotDetailsLookup,
	events EventPublisher,
) *SubmitReviewUseCase {
	return &SubmitReviewUseCase{getter: getter, owners: owners, reviews: reviews, spots: spots, events: events}
}

func (uc *SubmitReviewUseCase) Execute(
	ctx context.Context,
	bookingID, callerID string,
	targetType enum.ReviewTargetType,
	rating int,
	comment string,
) (entity.Review, error) {
	if rating < 1 || rating > 5 {
		return entity.Review{}, errors.NewInvalidInput("INVALID_RATING", "rating must be between 1 and 5")
	}

	booking, err := uc.getter.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Review{}, err
	}
	if booking.Status != enum.BookingStatusCompleted {
		return entity.Review{}, errors.NewConflict("la reserva todavía no terminó")
	}

	targetID, err := uc.resolveTarget(ctx, booking, callerID, targetType)
	if err != nil {
		return entity.Review{}, err
	}

	review := entity.Review{
		BookingID:  bookingID,
		ReviewerID: callerID,
		TargetType: targetType,
		TargetID:   targetID,
		Rating:     rating,
		Comment:    comment,
	}

	created, saved, err := uc.reviews.CreateReview(ctx, review)
	if err != nil {
		return entity.Review{}, err
	}
	if !created {
		return entity.Review{}, errors.NewConflict("ya calificaste esto")
	}
	uc.announce(ctx, saved, booking)
	return saved, nil
}

// announce tells the reviewed person they got a review (review.received). Only reviews of
// a person: the driver rates the spot and its owner together, so the owner hears about it
// once, from the owner review.
func (uc *SubmitReviewUseCase) announce(ctx context.Context, review entity.Review, booking entity.Booking) {
	if uc.events == nil {
		return
	}
	var recipientRole string
	switch review.TargetType {
	case enum.ReviewTargetOwner:
		recipientRole = RecipientRoleOwner
	case enum.ReviewTargetDriver:
		recipientRole = RecipientRoleDriver
	default:
		return
	}
	spot, _ := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	emitBookingEvent(ctx, uc.events, RoutingKeyReviewReceived, newReviewReceivedEvent(review, booking, spot, recipientRole))
}

// resolveTarget maps a target type to the user/spot id being rated, and checks that
// callerID is allowed to submit that particular review: the driver rates the spot or
// the owner, the owner rates the driver.
func (uc *SubmitReviewUseCase) resolveTarget(
	ctx context.Context,
	booking entity.Booking,
	callerID string,
	targetType enum.ReviewTargetType,
) (string, error) {
	switch targetType {
	case enum.ReviewTargetSpot:
		if callerID != booking.UserID {
			return "", errors.NewForbidden("solo el conductor puede calificar la cochera")
		}
		return booking.ParkingSpotID, nil

	case enum.ReviewTargetOwner:
		if callerID != booking.UserID {
			return "", errors.NewForbidden("solo el conductor puede calificar al propietario")
		}
		return uc.owners.GetSpotOwnerID(ctx, booking.ParkingSpotID)

	case enum.ReviewTargetDriver:
		ownerID, err := uc.owners.GetSpotOwnerID(ctx, booking.ParkingSpotID)
		if err != nil {
			return "", err
		}
		if callerID != ownerID {
			return "", errors.NewForbidden("solo el propietario puede calificar al conductor")
		}
		return booking.UserID, nil

	default:
		return "", errors.NewInvalidInput("INVALID_TARGET_TYPE", "targetType must be spot, owner, or driver")
	}
}
