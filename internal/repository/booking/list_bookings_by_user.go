package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListBookingsByUser returns all bookings for a given user_id, most recently *created* first
// -- not by start time, so a booking made just now for a far-future date still sorts above
// one made earlier for a sooner date.
//
// Checkouts the driver never paid are left out: EXPIRED rows, and PENDING_PAYMENT rows whose
// hold already elapsed but that the expiry job hasn't swept to EXPIRED yet. A still-live
// PENDING_PAYMENT (inside its hold window) is kept, so the driver can see it while paying.
func (repository *Repository) ListBookingsByUser(
	ctx context.Context,
	userID string,
) ([]entity.Booking, error) {
	query := `
		SELECT id, parking_spot_id, user_id,
		       lower(time_range), upper(time_range),
		       status, hold_expires_at, amount, COALESCE(payment_id, ''), paid_at,
		       cancellation_reason, kind,
		       verification_code, verification_attempts, verified_at, created_at, updated_at,
		       vehicle_plate, vehicle_brand, vehicle_model, vehicle_country
		FROM ` + constants.BookingsTableName + `
		WHERE user_id = $1
		  AND status <> 'EXPIRED'
		  AND NOT (status = 'PENDING_PAYMENT' AND hold_expires_at <= now())
		ORDER BY created_at DESC
	`

	rows, err := repository.database.QueryContext(ctx, query, userID)
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
			&b.VehiclePlate, &b.VehicleBrand, &b.VehicleModel, &b.VehicleCountry,
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
