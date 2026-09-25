package constants

const (
	// PayoutsTableName is the PostgreSQL table backing the payout repository.
	PayoutsTableName = "payouts"

	// PlatformSettingsTableName is the PostgreSQL table holding the single
	// platform_settings row (commission rate, etc -- see migrations/015).
	PlatformSettingsTableName = "platform_settings"

	// DefaultCommissionRateBP is the fallback commission rate (5.00%, in basis points).
	// migrations/015_create_platform_settings.sql seeds this same value into the single
	// platform_settings row, so this constant is only ever used as a defensive fallback if
	// that row were somehow missing, never as the actual source of truth.
	DefaultCommissionRateBP = 500

	// MaxCommissionRateBP is 100.00% expressed in basis points -- the upper bound accepted
	// by SetCommissionUseCase.
	MaxCommissionRateBP = 10000
)
