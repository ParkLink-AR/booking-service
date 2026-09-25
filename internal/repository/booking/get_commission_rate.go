package booking

import (
	"context"
	"database/sql"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// GetCommissionRate reads the platform's current commission settings off the single
// platform_settings row (see migrations/015_create_platform_settings.sql). Falling back to
// constants.DefaultCommissionRateBP only guards against that row somehow being missing --
// the migration seeds it, so this path shouldn't normally be hit.
func (repository *Repository) GetCommissionRate(ctx context.Context) (entity.PlatformSettings, error) {
	row := repository.database.QueryRowContext(ctx, `
		SELECT commission_rate_bp, updated_at, updated_by_admin_id
		FROM `+constants.PlatformSettingsTableName+`
		WHERE id = true
	`)

	var settings entity.PlatformSettings
	if err := row.Scan(&settings.CommissionRateBP, &settings.UpdatedAt, &settings.UpdatedByAdminID); err != nil {
		if err == sql.ErrNoRows {
			return entity.PlatformSettings{CommissionRateBP: constants.DefaultCommissionRateBP}, nil
		}
		return entity.PlatformSettings{}, errors.NewInternalError("booking.GetCommissionRate", err)
	}
	return settings, nil
}
