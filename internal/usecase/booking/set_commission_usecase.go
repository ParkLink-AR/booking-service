package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/set_commission_repository_mock.go -package=mocks -source=./set_commission_usecase.go

// CommissionRateSetter updates the platform's commission rate. Implemented by
// internal/repository/booking.
type CommissionRateSetter interface {
	SetCommissionRate(ctx context.Context, rateBP int, adminUserID string) (entity.PlatformSettings, error)
}

// SetCommissionUseCase powers PUT /admin/settings/commission. The rate is stored in basis
// points (0-10000, i.e. 0%-100%) -- see constants.MaxCommissionRateBP.
type SetCommissionUseCase struct {
	settings CommissionRateSetter
}

func NewSetCommissionUseCase(settings CommissionRateSetter) *SetCommissionUseCase {
	return &SetCommissionUseCase{settings: settings}
}

func (uc *SetCommissionUseCase) Execute(ctx context.Context, rateBP int, adminUserID string) (entity.PlatformSettings, error) {
	if rateBP < 0 || rateBP > constants.MaxCommissionRateBP {
		return entity.PlatformSettings{}, errors.NewInvalidInput("INVALID_COMMISSION_RATE", "commission_rate_bp must be between 0 and 10000")
	}
	return uc.settings.SetCommissionRate(ctx, rateBP, adminUserID)
}
