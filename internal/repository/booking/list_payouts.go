package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListPayouts returns every payout, newest first, optionally filtered by status -- powers
// GET /admin/payouts.
func (repository *Repository) ListPayouts(ctx context.Context, status *enum.PayoutStatus) ([]entity.Payout, error) {
	query := `
		SELECT id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount,
			status, paid_at, paid_by_admin_id, proof_url, created_at
		FROM ` + constants.PayoutsTableName + `
	`
	args := []any{}
	if status != nil {
		query += "WHERE status = $1\n"
		args = append(args, status.String())
	}
	query += "ORDER BY created_at DESC"

	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListPayouts", err)
	}
	defer rows.Close()

	var payouts []entity.Payout
	for rows.Next() {
		var p entity.Payout
		var statusLabel string
		if err := rows.Scan(
			&p.ID, &p.BookingID, &p.OwnerID, &p.PaymentID, &p.GrossAmount, &p.CommissionAmount, &p.NetAmount,
			&statusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.CreatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListPayouts", err)
		}
		p.Status, _ = parsePayoutStatus(statusLabel)
		payouts = append(payouts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListPayouts", err)
	}
	return payouts, nil
}
