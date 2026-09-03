package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/lib/pq"
)

// GetSpotRatings returns the average rating and review count for each of the given
// parking spot IDs that has at least one review. Spots with no reviews are simply
// absent from the returned map -- the caller treats that as 0/0.
func (repository *Repository) GetSpotRatings(ctx context.Context, spotIDs []string) (map[string]entity.RatingSummary, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT target_id, AVG(rating), COUNT(*)
		FROM `+constants.ReviewsTableName+`
		WHERE target_type = 'spot' AND target_id = ANY($1)
		GROUP BY target_id
	`, pq.Array(spotIDs))
	if err != nil {
		return nil, errors.NewInternalError("booking.GetSpotRatings", err)
	}
	defer rows.Close()

	ratings := make(map[string]entity.RatingSummary)
	for rows.Next() {
		var spotID string
		var summary entity.RatingSummary
		if err := rows.Scan(&spotID, &summary.Average, &summary.Count); err != nil {
			return nil, errors.NewInternalError("booking.GetSpotRatings", err)
		}
		ratings[spotID] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.GetSpotRatings", err)
	}
	return ratings, nil
}
