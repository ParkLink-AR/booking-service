package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreatePayout inserts a new PENDING payout row for a booking. Called once per eligible
// booking by GeneratePendingPayoutsUseCase -- booking_id is UNIQUE, so a booking can never
// end up with two payout rows.
func (repository *Repository) CreatePayout(ctx context.Context, payout entity.Payout) (entity.Payout, error) {
	row := repository.database.QueryRowContext(ctx, `
		INSERT INTO `+constants.PayoutsTableName+`
			(booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
			id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount,
			status, paid_at, paid_by_admin_id, proof_url, settlement_id, created_at
	`, payout.BookingID, payout.OwnerID, payout.PaymentID, payout.GrossAmount, payout.CommissionAmount, payout.NetAmount)

	var p entity.Payout
	var statusLabel string
	if err := row.Scan(
		&p.ID, &p.BookingID, &p.OwnerID, &p.PaymentID, &p.GrossAmount, &p.CommissionAmount, &p.NetAmount,
		&statusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.SettlementID, &p.CreatedAt,
	); err != nil {
		return entity.Payout{}, errors.NewInternalError("booking.CreatePayout", err)
	}
	p.Status, _ = parsePayoutStatus(statusLabel)
	return p, nil
}
