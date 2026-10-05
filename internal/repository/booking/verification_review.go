package booking

import (
	"context"
	"database/sql"
	stderrors "errors"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// verificationReviewColumns is the column list both queries below select/return, in the
// order scanVerificationReviewBooking expects.
const verificationReviewColumns = `
	id, parking_spot_id, user_id,
	lower(time_range), upper(time_range),
	status, amount, owner_amount, COALESCE(payment_id, ''), paid_at,
	kind, verified_at, verification_review, created_at, updated_at,
	vehicle_plate, vehicle_brand, vehicle_model, vehicle_country`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanVerificationReviewBooking(row rowScanner) (entity.Booking, error) {
	var b entity.Booking
	var statusLabel, kindLabel string
	var reviewLabel sql.NullString
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.Amount, &b.OwnerAmount, &b.PaymentID, &b.PaidAt,
		&kindLabel, &b.VerifiedAt, &reviewLabel, &b.CreatedAt, &b.UpdatedAt,
		&b.VehiclePlate, &b.VehicleBrand, &b.VehicleModel, &b.VehicleCountry,
	); err != nil {
		return entity.Booking{}, err
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	if reviewLabel.Valid {
		if review, ok := enum.ParseVerificationReview(reviewLabel.String); ok {
			b.VerificationReview = &review
		}
	}
	return b, nil
}

// ListBookingsPendingVerificationReview returns every booking whose verification_review is
// PENDING -- paid, completed, and never verified in person -- oldest stay first, for the
// admin panel's review queue.
func (repository *Repository) ListBookingsPendingVerificationReview(ctx context.Context) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT `+verificationReviewColumns+`
		FROM `+constants.BookingsTableName+`
		WHERE verification_review = 'PENDING'
		ORDER BY upper(time_range) ASC
	`)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsPendingVerificationReview", err)
	}
	defer rows.Close()

	var bookings []entity.Booking
	for rows.Next() {
		booking, err := scanVerificationReviewBooking(rows)
		if err != nil {
			return nil, errors.NewInternalError("booking.ListBookingsPendingVerificationReview", err)
		}
		bookings = append(bookings, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsPendingVerificationReview", err)
	}
	return bookings, nil
}

// ResolveVerificationReview atomically moves a booking's verification_review from PENDING
// to review (APPROVED or REFUNDED). It only touches rows still PENDING, so two admins
// resolving the same booking at once can't both succeed: the second gets a Conflict.
func (repository *Repository) ResolveVerificationReview(
	ctx context.Context,
	bookingID string,
	review enum.VerificationReview,
) (entity.Booking, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET    verification_review = $2,
		       updated_at          = now()
		WHERE  id = $1
		  AND  verification_review = 'PENDING'
		RETURNING `+verificationReviewColumns,
		bookingID, review.String())

	booking, err := scanVerificationReviewBooking(row)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return entity.Booking{}, errors.NewConflict("esta reserva no tiene una revisión pendiente")
		}
		return entity.Booking{}, errors.NewInternalError("booking.ResolveVerificationReview", err)
	}
	return booking, nil
}
