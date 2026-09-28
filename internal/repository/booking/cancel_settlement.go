package booking

import (
	"context"
	"database/sql"
	stderrors "errors"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// CancelSettlement undoes a mistaken draft before it's confirmed: releases every payout it
// covered back to unattached (settlement_id = NULL, immediately eligible for a future draft)
// and deletes the settlement row -- nothing referenced it as "paid" yet, so there's no history
// worth keeping. Returns ConflictError if the settlement has already been confirmed (PAID) --
// a confirmed settlement is a record of money that actually moved and can't be undone here --
// and NotFoundError if it doesn't exist. Locks the settlement row first so this can't race a
// concurrent ConfirmSettlement for the same id.
func (repository *Repository) CancelSettlement(ctx context.Context, settlementID string) error {
	tx, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.NewInternalError("booking.CancelSettlement", err)
	}
	defer func() { _ = tx.Rollback() }()

	var statusLabel string
	err = tx.QueryRowContext(ctx, `
		SELECT status FROM `+constants.SettlementsTableName+` WHERE id = $1 FOR UPDATE
	`, settlementID).Scan(&statusLabel)
	if stderrors.Is(err, sql.ErrNoRows) {
		return errors.NewNotFound("settlement", settlementID)
	}
	if err != nil {
		return errors.NewInternalError("booking.CancelSettlement", err)
	}
	status, _ := parseSettlementStatus(statusLabel)
	if status != enum.SettlementStatusPending {
		return errors.NewConflict("esta liquidación ya fue confirmada y no se puede cancelar")
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE `+constants.PayoutsTableName+` SET settlement_id = NULL WHERE settlement_id = $1
	`, settlementID); err != nil {
		return errors.NewInternalError("booking.CancelSettlement", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM `+constants.SettlementsTableName+` WHERE id = $1
	`, settlementID); err != nil {
		return errors.NewInternalError("booking.CancelSettlement", err)
	}

	if err := tx.Commit(); err != nil {
		return errors.NewInternalError("booking.CancelSettlement", err)
	}
	return nil
}
