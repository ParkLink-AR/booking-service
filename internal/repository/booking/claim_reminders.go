package booking

import (
	"context"
	"database/sql"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ClaimApprovalReminders marks and returns every booking request still waiting for its
// owner whose answer deadline is less than `within` away -- the owner gets one reminder
// before it expires unanswered (see ExpireStalePendingApprovals).
func (repository *Repository) ClaimApprovalReminders(ctx context.Context, within time.Duration) ([]entity.Booking, error) {
	return repository.claimReminders(ctx, "booking.ClaimApprovalReminders", `
		UPDATE `+constants.BookingsTableName+`
		SET    approval_reminder_sent_at = now()
		WHERE  status = 'PENDING_OWNER_APPROVAL'
		  AND  approval_reminder_sent_at IS NULL
		  AND  owner_response_deadline > now()
		  AND  owner_response_deadline <= now() + make_interval(secs => $1)
		RETURNING `+reminderColumns, within)
}

// ClaimStartReminders marks and returns every confirmed booking whose before-start
// reminder is due -- both sides get one. How far ahead depends on how far ahead the
// booking was made: half of it, between minLead and maxLead (booked 3h ahead -> 1h before;
// 30 min ahead -> 15 min before; 15 min ahead -> ~7 min before), so a last-minute booking
// still gets a useful reminder instead of one right on top of its confirmation.
func (repository *Repository) ClaimStartReminders(ctx context.Context, maxLead, minLead time.Duration) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    start_reminder_sent_at = now()
		WHERE  status = 'CONFIRMED'
		  AND  start_reminder_sent_at IS NULL
		  AND  lower(time_range) > now()
		  AND  lower(time_range) <= now() + LEAST(
		         make_interval(secs => $1),
		         GREATEST(make_interval(secs => $2), (lower(time_range) - created_at) / 2)
		       )
		RETURNING `+reminderColumns, maxLead.Seconds(), minLead.Seconds())
	if err != nil {
		return nil, errors.NewInternalError("booking.ClaimStartReminders", err)
	}
	return scanReminders(rows, "booking.ClaimStartReminders")
}

// ClaimEndReminders marks and returns every stay ("estadía") under way that ends less than
// `within` from now -- the driver gets one reminder before checking out.
func (repository *Repository) ClaimEndReminders(ctx context.Context, within time.Duration) ([]entity.Booking, error) {
	return repository.claimReminders(ctx, "booking.ClaimEndReminders", `
		UPDATE `+constants.BookingsTableName+`
		SET    end_reminder_sent_at = now()
		WHERE  status IN ('CONFIRMED', 'IN_PROGRESS')
		  AND  kind = 'STAY'
		  AND  end_reminder_sent_at IS NULL
		  AND  upper(time_range) > now()
		  AND  upper(time_range) <= now() + make_interval(secs => $1)
		RETURNING `+reminderColumns, within)
}

// reminderColumns is what a reminder needs to know about its booking.
const reminderColumns = `
	id, parking_spot_id, user_id, lower(time_range), upper(time_range),
	status, owner_response_deadline, amount, COALESCE(payment_id, '')`

func (repository *Repository) claimReminders(ctx context.Context, operation, query string, within time.Duration) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, query, within.Seconds())
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}
	return scanReminders(rows, operation)
}

func scanReminders(rows *sql.Rows, operation string) ([]entity.Booking, error) {
	defer rows.Close()

	var claimed []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel string
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID, &b.Start, &b.End,
			&statusLabel, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID,
		); err != nil {
			return nil, errors.NewInternalError(operation, err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		claimed = append(claimed, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}
	return claimed, nil
}
