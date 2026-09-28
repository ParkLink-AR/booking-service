package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListSettlementsByOwner returns ownerID's settlements, newest first, optionally filtered by
// status. Used both by the owner's own self-service "Finanzas" list (which always passes
// SettlementStatusPaid -- see usecase.ListSettlementsByOwnerUseCase, an owner has no reason to
// see a draft an admin hasn't confirmed yet) and available for admin use with no filter.
func (repository *Repository) ListSettlementsByOwner(ctx context.Context, ownerID string, status *enum.SettlementStatus) ([]entity.Settlement, error) {
	query := `
		SELECT id, owner_id, gross_amount, commission_amount, net_amount, status, proof_url,
			paid_at, paid_by_admin_id, created_by_admin_id, created_at
		FROM ` + constants.SettlementsTableName + `
		WHERE owner_id = $1
	`
	args := []any{ownerID}
	if status != nil {
		query += "AND status = $2\n"
		args = append(args, status.String())
	}
	query += "ORDER BY COALESCE(paid_at, created_at) DESC"

	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListSettlementsByOwner", err)
	}
	defer rows.Close()

	var settlements []entity.Settlement
	for rows.Next() {
		var s entity.Settlement
		var statusLabel string
		if err := rows.Scan(
			&s.ID, &s.OwnerID, &s.GrossAmount, &s.CommissionAmount, &s.NetAmount, &statusLabel, &s.ProofURL,
			&s.PaidAt, &s.PaidByAdminID, &s.CreatedByAdminID, &s.CreatedAt,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListSettlementsByOwner", err)
		}
		s.Status, _ = parseSettlementStatus(statusLabel)
		settlements = append(settlements, s)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListSettlementsByOwner", err)
	}
	return settlements, nil
}
