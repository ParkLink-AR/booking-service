package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListReviewsByTarget returns every review left for a spot, an owner, or a driver
// (identified by targetID), most recent first.
func (repository *Repository) ListReviewsByTarget(
	ctx context.Context,
	targetType enum.ReviewTargetType,
	targetID string,
) ([]entity.Review, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, booking_id, reviewer_id, target_type, target_id, rating, COALESCE(comment, ''), created_at
		FROM `+constants.ReviewsTableName+`
		WHERE target_type = $1 AND target_id = $2
		ORDER BY created_at DESC
	`, targetType.String(), targetID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListReviewsByTarget", err)
	}
	defer rows.Close()

	var reviews []entity.Review
	for rows.Next() {
		var r entity.Review
		var targetTypeLabel string
		if err := rows.Scan(
			&r.ID, &r.BookingID, &r.ReviewerID, &targetTypeLabel, &r.TargetID,
			&r.Rating, &r.Comment, &r.CreatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListReviewsByTarget", err)
		}
		r.TargetType, _ = enum.ParseReviewTargetType(targetTypeLabel)
		reviews = append(reviews, r)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListReviewsByTarget", err)
	}
	return reviews, nil
}

// ListReviewsByBooking returns every review (by anyone, for any target) tied to a
// single booking -- at most 3 rows (spot, owner, driver).
func (repository *Repository) ListReviewsByBooking(ctx context.Context, bookingID string) ([]entity.Review, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, booking_id, reviewer_id, target_type, target_id, rating, COALESCE(comment, ''), created_at
		FROM `+constants.ReviewsTableName+`
		WHERE booking_id = $1
	`, bookingID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListReviewsByBooking", err)
	}
	defer rows.Close()

	var reviews []entity.Review
	for rows.Next() {
		var r entity.Review
		var targetTypeLabel string
		if err := rows.Scan(
			&r.ID, &r.BookingID, &r.ReviewerID, &targetTypeLabel, &r.TargetID,
			&r.Rating, &r.Comment, &r.CreatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListReviewsByBooking", err)
		}
		r.TargetType, _ = enum.ParseReviewTargetType(targetTypeLabel)
		reviews = append(reviews, r)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListReviewsByBooking", err)
	}
	return reviews, nil
}
