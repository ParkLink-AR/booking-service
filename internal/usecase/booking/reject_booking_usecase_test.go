package booking_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestRejectBookingUseCaseSuite(t *testing.T) {
	suite.Run(t, new(rejectBookingUseCaseSuite))
}

type rejectBookingUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	rejecter   *mocks.MockBookingRejecter
	spots      *mocks.MockSpotDetailsLookup
	refunder   *mocks.MockPaymentRefunder
	events     *mocks.MockEventPublisher
	useCase    *booking.RejectBookingUseCase
	context    context.Context

	bookingID string
	ownerID   string
	driverID  string
}

func (s *rejectBookingUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.rejecter = mocks.NewMockBookingRejecter(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.refunder = mocks.NewMockPaymentRefunder(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewRejectBookingUseCase(s.rejecter, s.spots, s.refunder, s.events)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
	s.driverID = "22222222-2222-2222-2222-222222222222"
}

func (s *rejectBookingUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *rejectBookingUseCaseSuite) pending() entity.Booking {
	return entity.Booking{
		ID: s.bookingID, ParkingSpotID: "spot-1", UserID: s.driverID,
		Status: enum.BookingStatusPendingOwnerApproval, PaymentID: "pay-1",
	}
}

func (s *rejectBookingUseCaseSuite) TestExecute_OwnerRejects_RefundsAndTellsTheDriver() {
	rejected := s.pending()
	rejected.Status = enum.BookingStatusCancelled

	s.rejecter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pending(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera Palermo"}, nil)
	s.rejecter.EXPECT().RejectBooking(gomock.Any(), s.bookingID).Return(rejected, nil)
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCancelled, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal(booking.CancelledByOwnerRejected, event["cancelled_by"])
			s.Equal(s.driverID, event["driver_user_id"])
			s.Equal("Cochera Palermo", event["parking_spot_title"])
			return nil
		},
	)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "pay-1").Return(nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID)

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusCancelled, got.Status)
}

func (s *rejectBookingUseCaseSuite) TestExecute_NotTheOwner_ForbiddenNothingHappens() {
	s.rejecter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pending(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: s.ownerID}, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, "someone-else")

	s.True(errors.IsForbidden(err))
}
