package booking

import (
	"context"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListBookingsByOwnerSpots returns every booking (any status) for any of parkingSpotIDs,
// most recent first -- the generalization of ListBookingsBySpot for GET
// /admin/owners/{id}/bookings, which needs every reservation received across all of an
// owner's spots at once.
func (repository *Repository) ListBookingsByOwnerSpots(ctx context.Context, parkingSpotIDs []string) ([]entity.Booking, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT
			id, parking_spot_id, user_id,
			lower(time_range), upper(time_range),
			status, hold_expires_at, owner_response_deadline, amount, owner_amount, COALESCE(payment_id, ''), paid_at,
			kind, created_at, updated_at,
			vehicle_plate, vehicle_brand, vehicle_model, vehicle_country
		FROM `+constants.BookingsTableName+`
		WHERE parking_spot_id = ANY($1::uuid[])
		ORDER BY created_at DESC
	`, pq.Array(parkingSpotIDs))
	if err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsByOwnerSpots", err)
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
			&kindLabel, &b.CreatedAt, &b.UpdatedAt,
			&b.VehiclePlate, &b.VehicleBrand, &b.VehicleModel, &b.VehicleCountry,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListBookingsByOwnerSpots", err)
		}
		b.Status, _ = parseBookingStatus(statusLabel)
		b.Kind, _ = parseBookingKind(kindLabel)
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListBookingsByOwnerSpots", err)
	}
	return bookings, nil
}
