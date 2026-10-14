package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ExpireCompletedBookings transitions every CONFIRMED or IN_PROGRESS booking whose time
// window has already ended to COMPLETED. This is what makes a stay "over" -- it's the
// trigger the review prompts key off of. A paid booking that completes without the
// driver's in-person code ever having been verified (still CONFIRMED, verified_at NULL)
// is also flagged verification_review = 'PENDING', which holds back the owner's payout
// until an admin reviews it (see ListEligibleBookingsWithoutPayout and
// ResolveVerificationReview). Returns the bookings it completed, so the caller can
// announce each one.
func (repository *Repository) ExpireCompletedBookings(ctx context.Context) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    status              = 'COMPLETED'::booking_status,
		       verification_review = CASE
		                               WHEN verified_at IS NULL AND paid_at IS NOT NULL THEN 'PENDING'
		                               ELSE verification_review
		                             END,
		       updated_at          = now()
		WHERE  status IN ('CONFIRMED', 'IN_PROGRESS')
		  AND  upper(time_range) < now()
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, amount, COALESCE(payment_id, ''), paid_at, created_at, updated_at,
			EXISTS (
				SELECT 1 FROM `+constants.BookingsTableName+` child
				WHERE child.parent_booking_id = `+constants.BookingsTableName+`.id
				  AND child.status IN ('CONFIRMED', 'IN_PROGRESS')
			)
	`)
	if err != nil {
		return nil, errors.NewInternalError("booking.ExpireCompletedBookings", err)
	}
	defer rows.Close()

	var completed []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel string
		var extended bool
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID,
			&b.Start, &b.End,
			&statusLabel, &b.Amount, &b.PaymentID, &b.PaidAt, &b.CreatedAt, &b.UpdatedAt,
			&extended,
		); err != nil {
			return nil, errors.NewInternalError("booking.ExpireCompletedBookings", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		if extended {
			// Its paid extension carries on: only marks that it has one (see
			// CompleteFinishedBookingsUseCase, which leaves the review prompt to the extension).
			b.Extension = &entity.BookingExtension{}
		}
		completed = append(completed, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ExpireCompletedBookings", err)
	}
	return completed, nil
}
