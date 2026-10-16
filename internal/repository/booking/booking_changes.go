package booking

import (
	"context"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

const cancelledBookingColumns = `
	id, parking_spot_id, user_id,
	lower(time_range), upper(time_range),
	status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''), paid_at,
	cancellation_reason, kind, created_at, updated_at`

func scanCancelledBooking(scanner rowScanner) (entity.Booking, error) {
	var b entity.Booking
	var statusLabel, kindLabel string
	if err := scanner.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID, &b.PaidAt,
		&b.CancellationReason, &kindLabel, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return entity.Booking{}, err
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	return b, nil
}

// OwnerCancelBooking calls off a booking its owner had already accepted (CONFIRMED only),
// keeping the reason they gave. The driver's full refund is the caller's job.
func (repository *Repository) OwnerCancelBooking(ctx context.Context, bookingID, note string) (entity.Booking, error) {
	booking, err := scanCancelledBooking(repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET status              = 'CANCELLED'::booking_status,
		    cancellation_reason = $2,
		    cancellation_note   = $3,
		    updated_at          = now()
		WHERE id = $1 AND status = 'CONFIRMED'
		RETURNING `+cancelledBookingColumns, bookingID, constants.CancellationReasonOwnerCancelled, note))
	if err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	booking.CancellationNote = &note
	// The driver gets everything back, debts included: they owe those again.
	if err := repository.ReleaseDebts(ctx, bookingID); err != nil {
		return entity.Booking{}, err
	}
	return booking, nil
}

// CancelExtensions calls off what extends parentID once the booking itself was cancelled:
// one still awaiting payment simply expires; a paid one is CANCELLED with reason. Returns
// the paid ones, which the caller refunds.
func (repository *Repository) CancelExtensions(ctx context.Context, parentID, reason string) ([]entity.Booking, error) {
	expired, err := repository.database.QueryContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET status = 'EXPIRED'::booking_status, updated_at = now()
		WHERE parent_booking_id = $1 AND status = 'PENDING_PAYMENT'
		RETURNING id
	`, parentID)
	if err != nil {
		return nil, errors.NewInternalError("booking.CancelExtensions", err)
	}
	var released []string
	for expired.Next() {
		var id string
		if err := expired.Scan(&id); err != nil {
			expired.Close()
			return nil, errors.NewInternalError("booking.CancelExtensions", err)
		}
		released = append(released, id)
	}
	expired.Close()
	rows, err := repository.database.QueryContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET status = 'CANCELLED'::booking_status, cancellation_reason = $2, updated_at = now()
		WHERE parent_booking_id = $1 AND status = 'CONFIRMED'
		RETURNING `+cancelledBookingColumns, parentID, reason)
	if err != nil {
		return nil, errors.NewInternalError("booking.CancelExtensions", err)
	}
	defer rows.Close()
	var cancelled []entity.Booking
	for rows.Next() {
		booking, err := scanCancelledBooking(rows)
		if err != nil {
			return nil, errors.NewInternalError("booking.CancelExtensions", err)
		}
		cancelled = append(cancelled, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.CancelExtensions", err)
	}
	rows.Close()
	// Both kinds give back their whole payment, debts included: the driver owes those again.
	for _, extension := range cancelled {
		released = append(released, extension.ID)
	}
	if err := repository.releaseDebtsOf(ctx, "booking.CancelExtensions", released); err != nil {
		return nil, err
	}
	return cancelled, nil
}

// PropagateVerificationToExtensions marks parentID's paid extensions as verified once the
// parent is: the driver's arrival was checked in person, and an extension is the same stay
// going on, so there's no second code to check.
func (repository *Repository) PropagateVerificationToExtensions(ctx context.Context, parentID string) error {
	if _, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+` child
		SET verified_at = parent.verified_at, updated_at = now()
		FROM `+constants.BookingsTableName+` parent
		WHERE parent.id = $1
		  AND child.parent_booking_id = parent.id
		  AND parent.verified_at IS NOT NULL
		  AND child.verified_at IS NULL
		  AND child.status IN ('CONFIRMED', 'IN_PROGRESS')
	`, parentID); err != nil {
		return errors.NewInternalError("booking.PropagateVerificationToExtensions", err)
	}
	return nil
}

// CountOwnerCancellations counts the accepted bookings an owner called off on any of their
// spots: in total and over the last 90 days. The extensions that went with one don't count
// again.
func (repository *Repository) CountOwnerCancellations(ctx context.Context, spotIDs []string) (entity.OwnerCancellationStats, error) {
	var stats entity.OwnerCancellationStats
	if len(spotIDs) == 0 {
		return stats, nil
	}
	if err := repository.database.QueryRowContext(ctx, `
		SELECT count(*), count(*) FILTER (WHERE updated_at > now() - interval '90 days')
		FROM `+constants.BookingsTableName+`
		WHERE parking_spot_id::text = ANY($1)
		  AND status = 'CANCELLED'
		  AND cancellation_reason = $2
		  AND parent_booking_id IS NULL
	`, pq.Array(spotIDs), constants.CancellationReasonOwnerCancelled).Scan(&stats.Total, &stats.Last90Days); err != nil {
		return stats, errors.NewInternalError("booking.CountOwnerCancellations", err)
	}
	return stats, nil
}
