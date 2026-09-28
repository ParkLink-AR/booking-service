package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetOwnerPendingBalances returns one row per owner with at least one unattached payout --
// oldest-pending-first, so ParkLink staff can see which owners have been waiting longest.
// "Unattached" means settlement_id IS NULL: a payout stops counting here the moment a
// Settlement draft sweeps it up (see usecase.DraftSettlementUseCase), even before that
// settlement is confirmed -- once drafted it's spoken for, not available for a second draft.
// Powers GET /admin/owners/pending-balances.
func (repository *Repository) GetOwnerPendingBalances(ctx context.Context) ([]entity.OwnerPendingBalance, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT owner_id, SUM(net_amount), COUNT(*), MIN(created_at)
		FROM `+constants.PayoutsTableName+`
		WHERE status = 'PENDING' AND settlement_id IS NULL
		GROUP BY owner_id
		ORDER BY MIN(created_at) ASC
	`)
	if err != nil {
		return nil, errors.NewInternalError("booking.GetOwnerPendingBalances", err)
	}
	defer rows.Close()

	var balances []entity.OwnerPendingBalance
	for rows.Next() {
		var b entity.OwnerPendingBalance
		if err := rows.Scan(&b.OwnerID, &b.PendingNetAmount, &b.PendingCount, &b.OldestPendingAt); err != nil {
			return nil, errors.NewInternalError("booking.GetOwnerPendingBalances", err)
		}
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.GetOwnerPendingBalances", err)
	}
	return balances, nil
}
