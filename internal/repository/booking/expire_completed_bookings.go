package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ExpireCompletedBookings transitions every CONFIRMED or IN_PROGRESS booking whose time
// window has already ended to COMPLETED. This is what makes a stay "over" -- it's the
// trigger the review prompts key off of. A paid booking that completes without the
// driver's in-person code ever having been verified (still CONFIRMED, verified_at NULL)
// is also flagged verification_review = 'PENDING', which holds back the owner's payout
// until an admin reviews it (see ListEligibleBookingsWithoutPayout and
// ResolveVerificationReview). Returns the number of rows updated.
func (repository *Repository) ExpireCompletedBookings(ctx context.Context) (int64, error) {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    status              = 'COMPLETED'::booking_status,
		       verification_review = CASE
		                               WHEN verified_at IS NULL AND paid_at IS NOT NULL THEN 'PENDING'
		                               ELSE verification_review
		                             END,
		       updated_at          = now()
		WHERE  status IN ('CONFIRMED', 'IN_PROGRESS')
		  AND  upper(time_range) < now()
	`)
	if err != nil {
		return 0, errors.NewInternalError("booking.ExpireCompletedBookings", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, errors.NewInternalError("booking.ExpireCompletedBookings.RowsAffected", err)
	}
	return n, nil
}
