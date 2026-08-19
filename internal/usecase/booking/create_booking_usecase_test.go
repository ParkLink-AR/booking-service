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

func TestCreateBookingUseCaseSuite(t *testing.T) {
	suite.Run(t, new(createBookingUseCaseSuite))
}

type createBookingUseCaseSuite struct {
	suite.Suite
	controller     *gomock.Controller
	bookingCreator *mocks.MockBookingCreator
	useCase        *booking.CreateBookingUseCase
	context        context.Context
}

func (s *createBookingUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookingCreator = mocks.NewMockBookingCreator(s.controller)
	s.useCase = booking.NewCreateBookingUseCase(s.bookingCreator)
	s.context = context.Background()
}

func (s *createBookingUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *createBookingUseCaseSuite) TestExecute_DelegatesToBookingCreator() {
	input := entity.CreateBookingInput{
		ParkingSpotID: "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f",
		UserID:        "11111111-1111-1111-1111-111111111111",
		Start:         time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
		End:           time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC),
		Amount:        5000,
	}
	want := entity.Booking{ID: "created-id", ParkingSpotID: input.ParkingSpotID}

	s.bookingCreator.EXPECT().CreateBooking(gomock.Any(), input).Return(want, nil)

	got, err := s.useCase.Execute(s.context, input)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *createBookingUseCaseSuite) TestExecute_ServiceErrorPropagates() {
	input := entity.CreateBookingInput{ParkingSpotID: "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"}

	s.bookingCreator.EXPECT().
		CreateBooking(gomock.Any(), input).
		Return(entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking"))

	got, err := s.useCase.Execute(s.context, input)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsConflict(err))
}
