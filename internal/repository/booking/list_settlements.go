package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListSettlements returns every settlement across every owner, optionally filtered by status
// -- powers GET /admin/settlements, so admin-backend can show drafts awaiting confirmation
// ("Liquidaciones a confirmar") separately from confirmed history ("Liquidaciones pagadas").
// Ordered by paid_at (falling back to created_at for still-PENDING drafts, which have no
// paid_at yet) so the newest settlement of either kind sorts first.
func (repository *Repository) ListSettlements(ctx context.Context, status *enum.SettlementStatus) ([]entity.Settlement, error) {
	query := `
		SELECT id, owner_id, gross_amount, commission_amount, net_amount, status, proof_url,
			paid_at, paid_by_admin_id, created_by_admin_id, created_at
		FROM ` + constants.SettlementsTableName + `
	`
	args := []any{}
	if status != nil {
		query += "WHERE status = $1\n"
		args = append(args, status.String())
	}
	query += "ORDER BY COALESCE(paid_at, created_at) DESC"

	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListSettlements", err)
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
			return nil, errors.NewInternalError("booking.ListSettlements", err)
		}
		s.Status, _ = parseSettlementStatus(statusLabel)
		settlements = append(settlements, s)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListSettlements", err)
	}
	return settlements, nil
}
