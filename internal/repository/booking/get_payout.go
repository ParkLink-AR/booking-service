package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetPayoutByID fetches a single payout by id, regardless of status.
func (repository *Repository) GetPayoutByID(ctx context.Context, payoutID string) (entity.Payout, error) {
	row := repository.database.QueryRowContext(ctx, `
		SELECT id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount,
			status, paid_at, paid_by_admin_id, proof_url, settlement_id, created_at
		FROM `+constants.PayoutsTableName+`
		WHERE id = $1
	`, payoutID)

	var p entity.Payout
	var statusLabel string
	if err := row.Scan(
		&p.ID, &p.BookingID, &p.OwnerID, &p.PaymentID, &p.GrossAmount, &p.CommissionAmount, &p.NetAmount,
		&statusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.SettlementID, &p.CreatedAt,
	); err != nil {
		return entity.Payout{}, errors.NewNotFound("payout", payoutID)
	}
	p.Status, _ = parsePayoutStatus(statusLabel)
	return p, nil
}
