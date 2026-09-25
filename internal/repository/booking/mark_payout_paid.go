package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// MarkPayoutPaid transitions a payout from PENDING to PAID, stamping who did it and
// (optionally) a proof-of-transfer image URL. Reads the row first so it can tell a missing
// payout apart from one that's already PAID -- returns NotFoundError for the former and
// ConflictError for the latter (no double-paying), instead of a single ambiguous error.
func (repository *Repository) MarkPayoutPaid(ctx context.Context, payoutID, adminUserID string, proofURL *string) (entity.Payout, error) {
	existing, err := repository.GetPayoutByID(ctx, payoutID)
	if err != nil {
		return entity.Payout{}, err
	}
	if existing.Status != enum.PayoutStatusPending {
		return entity.Payout{}, errors.NewConflict("este pago ya fue marcado como pagado")
	}

	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.PayoutsTableName+`
		SET
			status           = 'PAID',
			paid_at          = now(),
			paid_by_admin_id = $2,
			proof_url        = $3
		WHERE id     = $1
		  AND status = 'PENDING'
		RETURNING
			id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount,
			status, paid_at, paid_by_admin_id, proof_url, created_at
	`, payoutID, adminUserID, proofURL)

	var p entity.Payout
	var statusLabel string
	if err := row.Scan(
		&p.ID, &p.BookingID, &p.OwnerID, &p.PaymentID, &p.GrossAmount, &p.CommissionAmount, &p.NetAmount,
		&statusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.CreatedAt,
	); err != nil {
		// Lost a race with another admin marking it paid between the check above and this
		// UPDATE -- rare, but treat it the same as the pre-check: already paid.
		return entity.Payout{}, errors.NewConflict("este pago ya fue marcado como pagado")
	}
	p.Status, _ = parsePayoutStatus(statusLabel)
	return p, nil
}
