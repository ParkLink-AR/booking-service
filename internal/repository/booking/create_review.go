package booking

import (
	"context"
	"database/sql"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateReview inserts a review and fills in its ID and CreatedAt. If the same reviewer
// already rated the same target for this booking (the table's UNIQUE constraint), no row
// is inserted and this returns (false, nil) instead of an error, so the caller can turn
// that into a 409.
func (repository *Repository) CreateReview(ctx context.Context, review entity.Review) (bool, entity.Review, error) {
	row := repository.database.QueryRowContext(ctx, `
		INSERT INTO `+constants.ReviewsTableName+` (booking_id, reviewer_id, target_type, target_id, rating, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (booking_id, reviewer_id, target_type) DO NOTHING
		RETURNING id, created_at
	`, review.BookingID, review.ReviewerID, review.TargetType.String(), review.TargetID, review.Rating, review.Comment)

	if err := row.Scan(&review.ID, &review.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return false, entity.Review{}, nil
		}
		return false, entity.Review{}, errors.NewInternalError("booking.CreateReview", err)
	}
	return true, review, nil
}
