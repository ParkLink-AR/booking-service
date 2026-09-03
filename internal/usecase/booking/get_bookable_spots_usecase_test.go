package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestGetBookableSpotsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(getBookableSpotsUseCaseSuite))
}

type getBookableSpotsUseCaseSuite struct {
	suite.Suite
	controller          *gomock.Controller
	bookableSpotsFinder *mocks.MockBookableSpotsFinder
	useCase             *booking.GetBookableSpotsUseCase
	context             context.Context
}

func (s *getBookableSpotsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookableSpotsFinder = mocks.NewMockBookableSpotsFinder(s.controller)
	s.useCase = booking.NewGetBookableSpotsUseCase(s.bookableSpotsFinder)
	s.context = context.Background()
}

func (s *getBookableSpotsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *getBookableSpotsUseCaseSuite) TestExecute_DelegatesToBookableSpotsFinder() {
	spots := []entity.SpotCapacity{{ID: "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f", Capacity: 1}}
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
	want := []string{"8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"}

	s.bookableSpotsFinder.EXPECT().GetBookableSpots(gomock.Any(), spots, start, end).Return(want, nil)

	got, err := s.useCase.Execute(s.context, spots, start, end)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *getBookableSpotsUseCaseSuite) TestExecute_ServiceErrorPropagates() {
	spots := []entity.SpotCapacity{{ID: "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f", Capacity: 1}}
	start := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)

	s.bookableSpotsFinder.EXPECT().
		GetBookableSpots(gomock.Any(), spots, start, end).
		Return(nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "start must be before end"))

	got, err := s.useCase.Execute(s.context, spots, start, end)

	s.Nil(got)
	s.True(errors.IsInvalidInput(err))
}
