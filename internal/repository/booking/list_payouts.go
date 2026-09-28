package booking

import (
	"context"
	"strconv"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListPayouts returns every payout, newest first, optionally filtered by status and/or owner
// id -- powers GET /admin/payouts.
func (repository *Repository) ListPayouts(ctx context.Context, status *enum.PayoutStatus, ownerID *string) ([]entity.Payout, error) {
	query := `
		SELECT id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount,
			status, paid_at, paid_by_admin_id, proof_url, settlement_id, created_at
		FROM ` + constants.PayoutsTableName + `
	`
	args := []any{}
	conditions := []string{}
	if status != nil {
		args = append(args, status.String())
		conditions = append(conditions, "status = $"+strconv.Itoa(len(args)))
	}
	if ownerID != nil {
		args = append(args, *ownerID)
		conditions = append(conditions, "owner_id = $"+strconv.Itoa(len(args)))
	}
	if len(conditions) > 0 {
		query += "WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			query += " AND " + c
		}
		query += "\n"
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
			&statusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.SettlementID, &p.CreatedAt,
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
