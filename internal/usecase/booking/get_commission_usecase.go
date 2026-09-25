package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_commission_repository_mock.go -package=mocks -source=./get_commission_usecase.go

// GetCommissionUseCase powers GET /admin/settings/commission.
type GetCommissionUseCase struct {
	settings CommissionRateGetter
}

func NewGetCommissionUseCase(settings CommissionRateGetter) *GetCommissionUseCase {
	return &GetCommissionUseCase{settings: settings}
}

func (uc *GetCommissionUseCase) Execute(ctx context.Context) (entity.PlatformSettings, error) {
	return uc.settings.GetCommissionRate(ctx)
}
