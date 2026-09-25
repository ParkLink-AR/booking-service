package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestGetCommissionUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getCommissionUseCaseSuite))
}

type getCommissionUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	settings   *mocks.MockCommissionRateGetter
	useCase    *booking.GetCommissionUseCase
	context    context.Context
}

func (s *getCommissionUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settings = mocks.NewMockCommissionRateGetter(s.controller)
	s.useCase = booking.NewGetCommissionUseCase(s.settings)
	s.context = context.Background()
}

func (s *getCommissionUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getCommissionUseCaseSuite) TestExecute_ReturnsWhateverTheRepositoryReturns() {
	want := entity.PlatformSettings{CommissionRateBP: 500}
	s.settings.EXPECT().GetCommissionRate(gomock.Any()).Return(want, nil)

	got, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(want, got)
}
