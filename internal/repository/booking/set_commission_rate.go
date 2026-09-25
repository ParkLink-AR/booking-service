package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// SetCommissionRate updates the platform's commission rate on the single platform_settings
// row. Range validation (0-10000 bp) is the use case's job, not the repository's.
func (repository *Repository) SetCommissionRate(ctx context.Context, rateBP int, adminUserID string) (entity.PlatformSettings, error) {
	row := repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.PlatformSettingsTableName+`
		SET commission_rate_bp  = $1,
		    updated_at          = now(),
		    updated_by_admin_id = $2
		WHERE id = true
		RETURNING commission_rate_bp, updated_at, updated_by_admin_id
	`, rateBP, adminUserID)

	var settings entity.PlatformSettings
	if err := row.Scan(&settings.CommissionRateBP, &settings.UpdatedAt, &settings.UpdatedByAdminID); err != nil {
		return entity.PlatformSettings{}, errors.NewInternalError("booking.SetCommissionRate", err)
	}
	return settings, nil
}
