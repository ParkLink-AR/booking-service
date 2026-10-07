package booking

import (
	"context"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListAccountBookings returns every booking that's still open (waiting for payment or for
// the owner, confirmed, or in progress) where userID is the driver or that's on one of
// spotIDs (the account's own spots) -- what closing that account has to deal with.
func (repository *Repository) ListAccountBookings(ctx context.Context, userID string, spotIDs []string) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, COALESCE(payment_id, ''), paid_at,
			kind, created_at, updated_at
		FROM `+constants.BookingsTableName+`
		WHERE status IN ('PENDING_PAYMENT', 'PENDING_OWNER_APPROVAL', 'CONFIRMED', 'IN_PROGRESS')
		  AND (user_id::text = $1 OR parking_spot_id::text = ANY($2))
		ORDER BY lower(time_range)
	`, userID, pq.Array(spotIDs))
	if err != nil {
		return nil, errors.NewInternalError("booking.ListAccountBookings", err)
	}
	defer rows.Close()

	var bookings []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel, kindLabel string
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID,
			&b.Start, &b.End,
			&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.PaymentID, &b.PaidAt,
			&kindLabel, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListAccountBookings", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		b.Kind, _ = parseBookingKind(kindLabel)
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListAccountBookings", err)
	}
	return bookings, nil
}

// CancelBookingWithReason transitions a CONFIRMED/PENDING_OWNER_APPROVAL booking to
// CANCELLED with the given cancellation reason. NotFound if it's no longer in one of
// those states (e.g. it was cancelled meanwhile).
func (repository *Repository) CancelBookingWithReason(ctx context.Context, bookingID, reason string) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET
			status              = 'CANCELLED'::booking_status,
			cancellation_reason = $2,
			updated_at          = now()
		WHERE id     = $1
		  AND status IN ('CONFIRMED', 'PENDING_OWNER_APPROVAL')
		RETURNING
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, amount, COALESCE(payment_id, ''), kind
	`, bookingID, reason)

	var b entity.Booking
	var statusLabel, kindLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.Amount, &b.PaymentID, &kindLabel,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	b.CancellationReason = &reason
	return b, nil
}

// ExpireHold drops a PENDING_PAYMENT booking's hold right away (EXPIRED), freeing its
// slot. A payment that still arrives for it is refunded by the confirm flow (HOLD_EXPIRED).
func (repository *Repository) ExpireHold(ctx context.Context, bookingID string) error {
	_, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    status = 'EXPIRED', updated_at = now()
		WHERE  id = $1 AND status = 'PENDING_PAYMENT'
	`, bookingID)
	if err != nil {
		return errors.NewInternalError("booking.ExpireHold", err)
	}
	return nil
}

// GetOwnerPendingPayouts is what ParkLink still owes this owner: payouts not paid yet
// (drafted into a settlement or not), plus completed paid bookings on their spots whose
// payout hasn't been generated yet (still under verification review, or not swept yet).
func (repository *Repository) GetOwnerPendingPayouts(ctx context.Context, ownerID string, spotIDs []string) (int, int, error) {
	var payoutCount, payoutAmount, bookingCount, bookingAmount int
	if err := repository.database.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(net_amount), 0)
		FROM `+constants.PayoutsTableName+`
		WHERE owner_id::text = $1 AND status = 'PENDING'
	`, ownerID).Scan(&payoutCount, &payoutAmount); err != nil {
		return 0, 0, errors.NewInternalError("booking.GetOwnerPendingPayouts", err)
	}
	if err := repository.database.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(COALESCE(b.owner_amount, b.amount)), 0)
		FROM `+constants.BookingsTableName+` b
		WHERE b.parking_spot_id::text = ANY($1)
		  AND b.status = 'COMPLETED'
		  AND COALESCE(b.payment_id, '') <> ''
		  AND COALESCE(b.verification_review, '') <> 'REFUNDED'
		  AND NOT EXISTS (SELECT 1 FROM `+constants.PayoutsTableName+` p WHERE p.booking_id = b.id)
	`, pq.Array(spotIDs)).Scan(&bookingCount, &bookingAmount); err != nil {
		return 0, 0, errors.NewInternalError("booking.GetOwnerPendingPayouts", err)
	}
	return payoutCount + bookingCount, payoutAmount + bookingAmount, nil
}

// ClearVehicleSnapshot wipes the vehicle (plate, brand, model) copied onto every booking
// this user made as a driver -- part of deleting their account's personal data.
func (repository *Repository) ClearVehicleSnapshot(ctx context.Context, userID string) error {
	_, err := repository.database.ExecContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    vehicle_plate = NULL, vehicle_brand = NULL, vehicle_model = NULL, vehicle_country = NULL
		WHERE  user_id::text = $1
	`, userID)
	if err != nil {
		return errors.NewInternalError("booking.ClearVehicleSnapshot", err)
	}
	return nil
}
