package booking

import (
	"context"
	"time"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// SeedDemoSpot stores a demo spot's finished booking and its reviews, and returns how many
// reviews it added. Both inserts skip what already exists (the booking by its id, a review by
// booking + reviewer), so loading the same spot again changes nothing.
//
// The booking is COMPLETED but never paid (paid_at stays NULL): the payout job only picks up paid
// bookings, so these never turn into payouts or settlements.
func (repository *Repository) SeedDemoSpot(ctx context.Context, seed entity.DemoSpotSeed) (int, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.NewInternalError("booking.SeedDemoSpot", err)
	}
	defer func() { _ = transaction.Rollback() }()

	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO `+constants.BookingsTableName+`
			(id, parking_spot_id, user_id, time_range, status, amount, owner_amount, kind, capacity, created_at, updated_at)
		VALUES ($1, $2, $3, tstzrange($4, $5, '[)'), 'COMPLETED', $6, $6, 'HOURLY', 1, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, seed.BookingID, seed.Spot.ID, seed.DriverID, seed.Start, seed.End, seed.AmountCents); err != nil {
		return 0, errors.NewInternalError("booking.SeedDemoSpot", err)
	}

	reviewerIDs := make([]string, len(seed.Reviews))
	ratings := make([]int64, len(seed.Reviews))
	comments := make([]string, len(seed.Reviews))
	createdAt := make([]string, len(seed.Reviews))
	for index, review := range seed.Reviews {
		reviewerIDs[index] = review.ReviewerID
		ratings[index] = int64(review.Rating)
		comments[index] = review.Comment
		createdAt[index] = review.CreatedAt.UTC().Format(time.RFC3339)
	}

	result, err := transaction.ExecContext(ctx, `
		INSERT INTO `+constants.ReviewsTableName+` (booking_id, reviewer_id, target_type, target_id, rating, comment, created_at)
		SELECT $1::uuid, reviewer, 'spot', $2::uuid, rating, NULLIF(comment, ''), created
		FROM unnest($3::uuid[], $4::smallint[], $5::text[], $6::timestamptz[]) AS demo(reviewer, rating, comment, created)
		ON CONFLICT (booking_id, reviewer_id, target_type) DO NOTHING
	`, seed.BookingID, seed.Spot.ID, pq.Array(reviewerIDs), pq.Array(ratings), pq.Array(comments), pq.Array(createdAt))
	if err != nil {
		return 0, errors.NewInternalError("booking.SeedDemoSpot", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, errors.NewInternalError("booking.SeedDemoSpot", err)
	}

	if err := transaction.Commit(); err != nil {
		return 0, errors.NewInternalError("booking.SeedDemoSpot", err)
	}
	return int(inserted), nil
}
