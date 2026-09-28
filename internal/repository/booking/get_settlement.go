package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetSettlementByID fetches a single settlement by id, regardless of status.
func (repository *Repository) GetSettlementByID(ctx context.Context, settlementID string) (entity.Settlement, error) {
	row := repository.database.QueryRowContext(ctx, `
		SELECT id, owner_id, gross_amount, commission_amount, net_amount, status, proof_url,
			paid_at, paid_by_admin_id, created_by_admin_id, created_at
		FROM `+constants.SettlementsTableName+`
		WHERE id = $1
	`, settlementID)

	var s entity.Settlement
	var statusLabel string
	if err := row.Scan(
		&s.ID, &s.OwnerID, &s.GrossAmount, &s.CommissionAmount, &s.NetAmount, &statusLabel, &s.ProofURL,
		&s.PaidAt, &s.PaidByAdminID, &s.CreatedByAdminID, &s.CreatedAt,
	); err != nil {
		return entity.Settlement{}, errors.NewNotFound("settlement", settlementID)
	}
	s.Status, _ = parseSettlementStatus(statusLabel)
	return s, nil
}
