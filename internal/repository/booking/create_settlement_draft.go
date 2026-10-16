package booking

import (
	"context"

	"github.com/lib/pq"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CreateSettlementDraft is phase 1 of settling an owner's payouts (see entity.Settlement for
// why this is split from confirmation): it freezes the amount owed right now, without
// asserting any money has moved yet. Atomically, in a single transaction: (a) SELECT ... FOR
// UPDATE every unattached (settlement_id IS NULL), PENDING payout row for ownerID -- the lock
// means a concurrent draft attempt for the same owner blocks until this transaction commits,
// then sees zero unattached rows left and gets NOTHING_TO_SETTLE instead of a second,
// overlapping draft; (b) if there are none, return NOTHING_TO_SETTLE; (c) sum gross/
// commission/net across them; (d) INSERT a new settlements row with status PENDING; (e)
// attach every locked payout to it (settlement_id set, but each payout's own status is left
// PENDING -- it only becomes PAID once the settlement is confirmed, see ConfirmSettlement).
func (repository *Repository) CreateSettlementDraft(ctx context.Context, ownerID, adminUserID string) (entity.Settlement, []entity.Payout, error) {
	tx, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.CreateSettlementDraft", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount, kind,
			status, paid_at, paid_by_admin_id, proof_url, settlement_id, created_at
		FROM `+constants.PayoutsTableName+`
		WHERE owner_id = $1 AND status = 'PENDING' AND settlement_id IS NULL
		ORDER BY created_at ASC
		FOR UPDATE
	`, ownerID)
	if err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.CreateSettlementDraft", err)
	}

	var payouts []entity.Payout
	for rows.Next() {
		var p entity.Payout
		var statusLabel string
		if err := rows.Scan(
			&p.ID, &p.BookingID, &p.OwnerID, &p.PaymentID, &p.GrossAmount, &p.CommissionAmount, &p.NetAmount, &p.Kind,
			&statusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.SettlementID, &p.CreatedAt,
		); err != nil {
			rows.Close()
			return entity.Settlement{}, nil, errors.NewInternalError("booking.CreateSettlementDraft", err)
		}
		p.Status, _ = parsePayoutStatus(statusLabel)
		payouts = append(payouts, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return entity.Settlement{}, nil, errors.NewInternalError("booking.CreateSettlementDraft", err)
	}
	rows.Close()

	if len(payouts) == 0 {
		return entity.Settlement{}, nil, errors.NewInvalidInput("NOTHING_TO_SETTLE", "el propietario no tiene pagos pendientes")
	}

	var gross, commission, net int
	payoutIDs := make([]string, len(payouts))
	for i, p := range payouts {
		gross += p.GrossAmount
		commission += p.CommissionAmount
		net += p.NetAmount
		payoutIDs[i] = p.ID
	}

	var settlement entity.Settlement
	var settlementStatusLabel string
	row := tx.QueryRowContext(ctx, `
		INSERT INTO `+constants.SettlementsTableName+`
			(owner_id, gross_amount, commission_amount, net_amount, created_by_admin_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id, owner_id, gross_amount, commission_amount, net_amount, status, proof_url,
			paid_at, paid_by_admin_id, created_by_admin_id, created_at
	`, ownerID, gross, commission, net, adminUserID)
	if err := row.Scan(
		&settlement.ID, &settlement.OwnerID, &settlement.GrossAmount, &settlement.CommissionAmount, &settlement.NetAmount,
		&settlementStatusLabel, &settlement.ProofURL, &settlement.PaidAt, &settlement.PaidByAdminID,
		&settlement.CreatedByAdminID, &settlement.CreatedAt,
	); err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.CreateSettlementDraft", err)
	}
	settlement.Status, _ = parseSettlementStatus(settlementStatusLabel)

	if _, err := tx.ExecContext(ctx, `
		UPDATE `+constants.PayoutsTableName+`
		SET settlement_id = $1
		WHERE id = ANY($2::uuid[])
	`, settlement.ID, pq.Array(payoutIDs)); err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.CreateSettlementDraft", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.CreateSettlementDraft", err)
	}

	for i := range payouts {
		payouts[i].SettlementID = &settlement.ID
	}

	return settlement, payouts, nil
}
