package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListEligibleBookingsWithoutPayout returns every COMPLETED, actually-paid booking whose
// time window ended at least 24h ago and that doesn't already have a payout row -- the pool
// GeneratePendingPayoutsUseCase turns into new PENDING payouts on each sweep. The LEFT JOIN
// against payouts (rather than a NOT EXISTS/subquery) plus `p.id IS NULL` is what makes this
// idempotent: a booking that already got a payout on a previous tick never comes back here.
func (repository *Repository) ListEligibleBookingsWithoutPayout(ctx context.Context) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT
			b.id, b.parking_spot_id, b.user_id,
			lower(b.time_range), upper(b.time_range),
			b.status, b.hold_expires_at, b.owner_response_deadline, b.amount, b.owner_amount, COALESCE(b.payment_id, ''), b.paid_at,
			b.cancellation_reason, b.kind, b.verification_code, b.verification_attempts, b.verified_at,
			b.created_at, b.updated_at
		FROM `+constants.BookingsTableName+` b
		LEFT JOIN `+constants.PayoutsTableName+` p ON p.booking_id = b.id
		WHERE b.status = 'COMPLETED'
		  AND b.paid_at IS NOT NULL
		  AND upper(b.time_range) <= now() - interval '24 hours'
		  AND p.id IS NULL
		ORDER BY b.created_at ASC
	`)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListEligibleBookingsWithoutPayout", err)
	}
	defer rows.Close()

	var bookings []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel, kindLabel string
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID,
			&b.Start, &b.End,
			&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.OwnerAmount, &b.PaymentID, &b.PaidAt,
			&b.CancellationReason, &kindLabel, &b.VerificationCode, &b.VerificationAttempts, &b.VerifiedAt,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListEligibleBookingsWithoutPayout", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		b.Kind, _ = parseBookingKind(kindLabel)
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListEligibleBookingsWithoutPayout", err)
	}
	return bookings, nil
}
