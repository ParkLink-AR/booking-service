package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ExpireStalePendingApprovals cancels every PENDING_OWNER_APPROVAL booking whose owner
// never responded within its deadline. Unlike ExpireStaleHolds (which releases an unpaid
// hold), these bookings were already paid, so the caller has to refund each one returned
// here -- that's why this returns the full rows (payment_id included) instead of a count.
func (repository *Repository) ExpireStalePendingApprovals(ctx context.Context) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status                  = 'CANCELLED'::booking_status,
			owner_response_deadline = NULL,
			updated_at              = now()
		WHERE status                  = 'PENDING_OWNER_APPROVAL'
		  AND owner_response_deadline < now()
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''),
			created_at, updated_at
	`)
	if err != nil {
		return nil, errors.NewInternalError("booking.ExpireStalePendingApprovals", err)
	}
	defer rows.Close()

	var expired []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel string
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID,
			&b.Start, &b.End,
			&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ExpireStalePendingApprovals", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		expired = append(expired, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ExpireStalePendingApprovals", err)
	}
	return expired, nil
}
