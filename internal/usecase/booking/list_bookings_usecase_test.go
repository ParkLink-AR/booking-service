package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestListBookingsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(listBookingsUseCaseSuite))
}

type listBookingsUseCaseSuite struct {
	suite.Suite
	controller    *gomock.Controller
	bookingLister *mocks.MockBookingLister
	useCase       *booking.ListBookingsUseCase
	context       context.Context
	parkingSpotID string
}

func (s *listBookingsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookingLister = mocks.NewMockBookingLister(s.controller)
	s.useCase = booking.NewListBookingsUseCase(s.bookingLister)
	s.context = context.Background()
	s.parkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *listBookingsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *listBookingsUseCaseSuite) TestExecute_DelegatesToBookingService() {
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	status := enum.BookingStatusConfirmed
	want := []entity.DayBookings{{Date: from}}

	s.bookingLister.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, &from, (*time.Time)(nil), &status, false).
		Return(want, nil)

	got, err := s.useCase.Execute(s.context, s.parkingSpotID, &from, nil, &status, false)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *listBookingsUseCaseSuite) TestExecute_ServiceErrorPropagates() {
	s.bookingLister.EXPECT().
		ListBookings(gomock.Any(), s.parkingSpotID, (*time.Time)(nil), (*time.Time)(nil), (*enum.BookingStatus)(nil), false).
		Return(nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "from must be before to"))

	got, err := s.useCase.Execute(s.context, s.parkingSpotID, nil, nil, nil, false)

	s.Nil(got)
	s.True(errors.IsInvalidInput(err))
	s.Equal("INVALID_DATE_RANGE", errors.InvalidInputCode(err))
}
