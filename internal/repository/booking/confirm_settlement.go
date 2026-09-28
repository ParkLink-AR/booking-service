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

// ConfirmSettlement is phase 2 of settling an owner's payouts (see entity.Settlement): this
// is the step that asserts the bank transfer actually went out, and the only one that stamps
// PaidAt/PaidByAdminID/ProofURL and flips both the settlement's and its payouts' Status to
// PAID. Locks the settlement row first so a concurrent confirm attempt for the same
// settlement can't both succeed -- the loser sees a non-PENDING status and gets a
// ConflictError instead of double-confirming. Every payout the draft attached to this
// settlement gets the same paid_at/paid_by_admin_id/proof_url stamped onto it, mirroring the
// settlement's own values, so a payout row is fully self-describing without a join.
func (repository *Repository) ConfirmSettlement(
	ctx context.Context, settlementID, adminUserID string, proofURL *string,
) (entity.Settlement, []entity.Payout, error) {
	tx, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.ConfirmSettlement", err)
	}
	defer func() { _ = tx.Rollback() }()

	var settlement entity.Settlement
	var statusLabel string
	err = tx.QueryRowContext(ctx, `
		SELECT id, owner_id, gross_amount, commission_amount, net_amount, status, proof_url,
			paid_at, paid_by_admin_id, created_by_admin_id, created_at
		FROM `+constants.SettlementsTableName+`
		WHERE id = $1
		FOR UPDATE
	`, settlementID).Scan(
		&settlement.ID, &settlement.OwnerID, &settlement.GrossAmount, &settlement.CommissionAmount, &settlement.NetAmount,
		&statusLabel, &settlement.ProofURL, &settlement.PaidAt, &settlement.PaidByAdminID,
		&settlement.CreatedByAdminID, &settlement.CreatedAt,
	)
	if stderrors.Is(err, sql.ErrNoRows) {
		return entity.Settlement{}, nil, errors.NewNotFound("settlement", settlementID)
	}
	if err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.ConfirmSettlement", err)
	}
	settlement.Status, _ = parseSettlementStatus(statusLabel)
	if settlement.Status != enum.SettlementStatusPending {
		return entity.Settlement{}, nil, errors.NewConflict("esta liquidación ya fue confirmada")
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE `+constants.SettlementsTableName+`
		SET status = 'PAID', paid_at = now(), paid_by_admin_id = $2, proof_url = $3
		WHERE id = $1
		RETURNING
			id, owner_id, gross_amount, commission_amount, net_amount, status, proof_url,
			paid_at, paid_by_admin_id, created_by_admin_id, created_at
	`, settlementID, adminUserID, proofURL)
	if err := row.Scan(
		&settlement.ID, &settlement.OwnerID, &settlement.GrossAmount, &settlement.CommissionAmount, &settlement.NetAmount,
		&statusLabel, &settlement.ProofURL, &settlement.PaidAt, &settlement.PaidByAdminID,
		&settlement.CreatedByAdminID, &settlement.CreatedAt,
	); err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.ConfirmSettlement", err)
	}
	settlement.Status, _ = parseSettlementStatus(statusLabel)

	rows, err := tx.QueryContext(ctx, `
		UPDATE `+constants.PayoutsTableName+`
		SET status = 'PAID', paid_at = $2, paid_by_admin_id = $3, proof_url = $4
		WHERE settlement_id = $1
		RETURNING
			id, booking_id, owner_id, payment_id, gross_amount, commission_amount, net_amount,
			status, paid_at, paid_by_admin_id, proof_url, settlement_id, created_at
	`, settlementID, settlement.PaidAt, adminUserID, proofURL)
	if err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.ConfirmSettlement", err)
	}

	var payouts []entity.Payout
	for rows.Next() {
		var p entity.Payout
		var payoutStatusLabel string
		if err := rows.Scan(
			&p.ID, &p.BookingID, &p.OwnerID, &p.PaymentID, &p.GrossAmount, &p.CommissionAmount, &p.NetAmount,
			&payoutStatusLabel, &p.PaidAt, &p.PaidByAdminID, &p.ProofURL, &p.SettlementID, &p.CreatedAt,
		); err != nil {
			rows.Close()
			return entity.Settlement{}, nil, errors.NewInternalError("booking.ConfirmSettlement", err)
		}
		p.Status, _ = parsePayoutStatus(payoutStatusLabel)
		payouts = append(payouts, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return entity.Settlement{}, nil, errors.NewInternalError("booking.ConfirmSettlement", err)
	}
	rows.Close()

	if err := tx.Commit(); err != nil {
		return entity.Settlement{}, nil, errors.NewInternalError("booking.ConfirmSettlement", err)
	}

	return settlement, payouts, nil
}
