package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetByID fetches a single booking by id, regardless of status.
func (repository *Repository) GetByID(ctx context.Context, bookingID string) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		SELECT
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, owner_amount, COALESCE(payment_id, ''), paid_at,
			cancellation_reason, kind, verification_code, verification_attempts, verified_at, created_at, updated_at,
			vehicle_plate, vehicle_brand, vehicle_model, vehicle_country, vehicle_type,
			refunded_amount, parent_booking_id, cancellation_note, debt_amount,
			extension.ext_id, extension.ext_end, extension.ext_status
		FROM `+constants.BookingsTableName+` b
		LEFT JOIN LATERAL (
			SELECT child.id AS ext_id, upper(child.time_range) AS ext_end, child.status::text AS ext_status
			FROM `+constants.BookingsTableName+` child
			WHERE child.parent_booking_id = b.id
			  AND (child.status IN ('CONFIRMED', 'IN_PROGRESS', 'COMPLETED')
			       OR (child.status = 'PENDING_PAYMENT' AND child.hold_expires_at > now()))
			ORDER BY child.created_at DESC
			LIMIT 1
		) extension ON true
		WHERE b.id = $1
	`, bookingID)

	var b entity.Booking
	var statusLabel, kindLabel string
	var extensionID, extensionStatus *string
	var extensionEnd *time.Time
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.OwnerAmount, &b.PaymentID, &b.PaidAt,
		&b.CancellationReason, &kindLabel, &b.VerificationCode, &b.VerificationAttempts, &b.VerifiedAt, &b.CreatedAt, &b.UpdatedAt,
		&b.VehiclePlate, &b.VehicleBrand, &b.VehicleModel, &b.VehicleCountry, &b.VehicleType,
		&b.RefundedAmount, &b.ParentBookingID, &b.CancellationNote, &b.DebtAmount,
		&extensionID, &extensionEnd, &extensionStatus,
	); err != nil {
		return entity.Booking{}, errors.NewNotFound("booking", bookingID)
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	if extensionID != nil && extensionEnd != nil && extensionStatus != nil {
		extensionState, _ := parseBookingStatus(*extensionStatus)
		b.Extension = &entity.BookingExtension{ID: *extensionID, End: *extensionEnd, Status: extensionState}
	}
	return b, nil
}
