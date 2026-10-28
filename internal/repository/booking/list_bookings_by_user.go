package booking

import (
	"context"
	"fmt"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// orderByRecent: most recently *created* first.
const orderByRecent = `created_at DESC`

// orderByUpcoming: first the bookings still ahead or under way -- active and not over yet --
// soonest start first; then everything else (finished, cancelled) by start, latest first.
const orderByUpcoming = `
	(status IN ('PENDING_PAYMENT', 'PENDING_OWNER_APPROVAL', 'CONFIRMED', 'IN_PROGRESS')
	 AND upper(time_range) > now()) DESC,
	CASE WHEN status IN ('PENDING_PAYMENT', 'PENDING_OWNER_APPROVAL', 'CONFIRMED', 'IN_PROGRESS')
	      AND upper(time_range) > now()
	     THEN lower(time_range) END ASC,
	lower(time_range) DESC,
	created_at DESC`

// ListBookingsByUser returns all bookings for a given user_id -- only the ones in status when
// it's not nil -- in the given order:
//   - UserBookingsOrderRecent: most recently *created* first -- not by start time, so a booking
//     made just now for a far-future date still sorts above one made earlier for a sooner date.
//   - UserBookingsOrderUpcoming: the active bookings that haven't ended yet first, the soonest
//     to start on top (one already under way sorts above all of them); then the rest
//     (finished, cancelled) by start time, latest first.
//
// Checkouts the driver never paid are left out: EXPIRED rows, and PENDING_PAYMENT rows whose
// hold already elapsed but that the expiry job hasn't swept to EXPIRED yet. A still-live
// PENDING_PAYMENT (inside its hold window) is kept, so the driver can see it while paying.
func (repository *Repository) ListBookingsByUser(
	ctx context.Context,
	userID string,
	order enum.UserBookingsOrder,
	status *enum.BookingStatus,
) ([]entity.Booking, error) {
	orderBy := orderByRecent
	if order == enum.UserBookingsOrderUpcoming {
		orderBy = orderByUpcoming
	}

	query := `
		SELECT id, parking_spot_id, user_id,
		       lower(time_range), upper(time_range),
		       status, hold_expires_at, amount, COALESCE(payment_id, ''), paid_at,
		       cancellation_reason, kind,
		       verification_code, verification_attempts, verified_at, created_at, updated_at,
		       vehicle_plate, vehicle_brand, vehicle_model, vehicle_country, vehicle_type
		FROM ` + constants.BookingsTableName + `
		WHERE user_id = $1
		  AND status <> 'EXPIRED'
		  AND NOT (status = 'PENDING_PAYMENT' AND hold_expires_at <= now())`
	args := []any{userID}
	if status != nil {
		args = append(args, status.String())
		query += fmt.Sprintf(" AND status = $%d::booking_status", len(args))
	}
	query += " ORDER BY " + orderBy

	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsByUser", err)
	}
	defer rows.Close()

	var bookings []entity.Booking
	for rows.Next() {
		var b entity.Booking
		var statusLabel, kindLabel string
		if err := rows.Scan(
			&b.ID, &b.ParkingSpotID, &b.UserID,
			&b.Start, &b.End,
			&statusLabel, &b.HoldExpiresAt, &b.Amount, &b.PaymentID, &b.PaidAt,
			&b.CancellationReason, &kindLabel, &b.VerificationCode, &b.VerificationAttempts, &b.VerifiedAt, &b.CreatedAt, &b.UpdatedAt,
			&b.VehiclePlate, &b.VehicleBrand, &b.VehicleModel, &b.VehicleCountry, &b.VehicleType,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListBookingsByUser", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		b.Kind, _ = parseBookingKind(kindLabel)
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsByUser", err)
	}
	return bookings, nil
}
