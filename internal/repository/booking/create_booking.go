package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateBooking inserts a new booking and returns it with its generated id and timestamps.
//
// There is no EXCLUDE constraint anymore (a capacity-N spot allows up to N overlapping
// active bookings). Instead this runs in a transaction that first takes a per-spot
// advisory lock, then recounts the concurrent active bookings in the requested range: if
// that already reaches booking.Capacity the insert is rejected as a conflict. The lock
// serialises concurrent creates for the same spot, so the count-then-insert is atomic. An
// extension (ParentBookingID set) doesn't count the booking it continues against itself.
func (repository *Repository) CreateBooking(ctx context.Context, booking entity.Booking) (entity.Booking, error) {
	capacity := booking.Capacity
	if capacity < 1 {
		capacity = 1
	}

	tx, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Serialise booking creation for this spot so the recount below can't race another insert.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, booking.ParkingSpotID); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}

	// Every existing booking blocks its range padded by the grace period (see
	// entity.OccupancyWindow), so the overlap filter is widened the same way.
	rows, err := tx.QueryContext(ctx, `
		SELECT lower(time_range), upper(time_range)
		FROM `+constants.BookingsTableName+`
		WHERE parking_spot_id = $1
		  AND (
		    status IN ('CONFIRMED', 'IN_PROGRESS', 'PENDING_OWNER_APPROVAL')
		    OR (status = 'PENDING_PAYMENT' AND hold_expires_at > now())
		  )
		  AND time_range && tstzrange($2, $3, '[)')
		  AND ($4::uuid IS NULL OR id <> $4::uuid)
	`, booking.ParkingSpotID, booking.Start.Add(-entity.GracePeriod), booking.End.Add(entity.GracePeriod), booking.ParentBookingID)
	if err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}

	var active []entity.TimeWindow
	for rows.Next() {
		var start, end time.Time
		if err := rows.Scan(&start, &end); err != nil {
			rows.Close()
			return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
		}
		active = append(active, entity.OccupancyWindow(entity.TimeWindow{Start: start, End: end}))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}
	rows.Close()

	if entity.MaxConcurrency(active, booking.Start, booking.End) >= capacity {
		return entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking")
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO `+constants.BookingsTableName+`
			(parking_spot_id, user_id, time_range, status, hold_expires_at, amount, owner_amount, capacity, kind,
			 vehicle_plate, vehicle_brand, vehicle_model, vehicle_country, vehicle_type, parent_booking_id)
		VALUES ($1, $2, tstzrange($3, $4, '[)'), $5::booking_status, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at
	`, booking.ParkingSpotID, booking.UserID, booking.Start, booking.End, booking.Status.String(), booking.HoldExpiresAt, booking.Amount, booking.OwnerAmount, capacity, booking.Kind.String(),
		booking.VehiclePlate, booking.VehicleBrand, booking.VehicleModel, booking.VehicleCountry, booking.VehicleType, booking.ParentBookingID)

	var id string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &createdAt, &updatedAt); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}

	// The driver's pending balances ride on this booking: its payment will include them (and they
	// go back to PENDING if its hold runs out). Taken in the same transaction, so two bookings
	// created at once can't both charge the same debt.
	var debt int
	if err := tx.QueryRowContext(ctx, `
		WITH taken AS (
			UPDATE `+constants.BookingDebtsTableName+`
			SET    status = 'RESERVED', charged_booking_id = $1
			WHERE  user_id = $2 AND status = 'PENDING'
			RETURNING amount
		)
		SELECT COALESCE(SUM(amount), 0) FROM taken
	`, id, booking.UserID).Scan(&debt); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}
	if debt > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE `+constants.BookingsTableName+` SET debt_amount = $2 WHERE id = $1`, id, debt); err != nil {
			return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return entity.Booking{}, errors.NewInternalError("booking.CreateBooking", err)
	}

	booking.ID = id
	booking.DebtAmount = debt
	booking.Capacity = capacity
	booking.CreatedAt = createdAt
	booking.UpdatedAt = updatedAt
	return booking, nil
}
