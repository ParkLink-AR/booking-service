package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestSetCommissionUseCaseSuite(t *testing.T) {
	suite.Run(t, new(setCommissionUseCaseSuite))
}

type setCommissionRangeCase struct {
	name        string
	rateBP      int
	expectValid bool
}

var setCommissionRangeCases = []setCommissionRangeCase{
	{name: "negative rate is rejected", rateBP: -1, expectValid: false},
	{name: "zero is the minimum valid rate", rateBP: 0, expectValid: true},
	{name: "500bp (5%) is valid", rateBP: 500, expectValid: true},
	{name: "10000bp (100%) is the maximum valid rate", rateBP: 10000, expectValid: true},
	{name: "10001bp is rejected", rateBP: 10001, expectValid: false},
	{name: "far out of range is rejected", rateBP: 999999, expectValid: false},
}

type setCommissionUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	settings   *mocks.MockCommissionRateSetter
	useCase    *booking.SetCommissionUseCase
	context    context.Context
	adminID    string
}

func (s *setCommissionUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.settings = mocks.NewMockCommissionRateSetter(s.controller)
	s.useCase = booking.NewSetCommissionUseCase(s.settings)
	s.context = context.Background()
	s.adminID = "44444444-4444-4444-4444-444444444444"
}

func (s *setCommissionUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *setCommissionUseCaseSuite) TestExecute_RangeValidation() {
	for _, tc := range setCommissionRangeCases {
		if tc.expectValid {
			s.settings.EXPECT().SetCommissionRate(gomock.Any(), tc.rateBP, s.adminID).
				Return(entity.PlatformSettings{CommissionRateBP: tc.rateBP}, nil)
		}

		got, err := s.useCase.Execute(s.context, tc.rateBP, s.adminID)

		if tc.expectValid {
			s.Require().NoError(err, tc.name)
			s.Equal(tc.rateBP, got.CommissionRateBP, tc.name)
		} else {
			s.True(errors.IsInvalidInput(err), tc.name)
			s.Equal(entity.PlatformSettings{}, got, tc.name)
		}
	}
}

func (s *setCommissionUseCaseSuite) TestExecute_RepositoryError_Propagates() {
	s.settings.EXPECT().SetCommissionRate(gomock.Any(), 500, s.adminID).
		Return(entity.PlatformSettings{}, errors.NewInternalError("boom", context.DeadlineExceeded))

	got, err := s.useCase.Execute(s.context, 500, s.adminID)

	s.Equal(entity.PlatformSettings{}, got)
	s.Error(err)
}
