package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListPayoutsBySettlement returns every payout a settlement covers, oldest first -- used to
// build the "payouts" list in a settlement's detail response (see
// controller/booking.settlementResponse).
func (repository *Repository) ListPayoutsBySettlement(ctx context.Context, settlementID string) ([]entity.Payout, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount,
			status, paid_at, paid_by_admin_id, proof_url, settlement_id, created_at
		FROM `+constants.PayoutsTableName+`
		WHERE settlement_id = $1
		ORDER BY created_at ASC
	`, settlementID)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListPayoutsBySettlement", err)
	}
	defer rows.Close()

	var payouts []entity.Payout
	for rows.Next() {
		var p entity.Payout
		var statusLabel string
		if err := rows.Scan(
			&p.ID, &p.BookingID, &p.OwnerID, &p.PaymentID, &p.GrossAmount, &p.CommissionAmount, &p.NetAmount,
			&statusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.SettlementID, &p.CreatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListPayoutsBySettlement", err)
		}
		p.Status, _ = parsePayoutStatus(statusLabel)
		payouts = append(payouts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListPayoutsBySettlement", err)
	}
	return payouts, nil
}
