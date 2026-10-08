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

func TestExpirePendingApprovalsUseCaseSuite(t *testing.T) {
	suite.Run(t, new(expirePendingApprovalsUseCaseSuite))
}

type expirePendingApprovalsUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	bookings   *mocks.MockStalePendingApprovalExpirer
	refunder   *mocks.MockPaymentRefunder
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.ExpirePendingApprovalsUseCase
	context    context.Context
}

func (s *expirePendingApprovalsUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.bookings = mocks.NewMockStalePendingApprovalExpirer(s.controller)
	s.refunder = mocks.NewMockPaymentRefunder(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewExpirePendingApprovalsUseCase(s.bookings, s.refunder, s.spots, s.events)
	s.context = context.Background()
}

func (s *expirePendingApprovalsUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *expirePendingApprovalsUseCaseSuite) TestExecute_RefundsAndTellsEveryDriver() {
	expired := []entity.Booking{
		{ID: "b1", ParkingSpotID: "spot-1", UserID: "driver-1", PaymentID: "pay-1"},
		{ID: "b2", ParkingSpotID: "spot-2", UserID: "driver-2", PaymentID: "pay-2"},
	}
	s.bookings.EXPECT().ExpireStalePendingApprovals(gomock.Any()).Return(expired, nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "pay-1").Return(nil)
	// One refund failing doesn't stop the other booking's refund or either notice.
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "pay-2").Return(errors.NewInternalError("test", context.DeadlineExceeded))
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{OwnerID: "owner-1", Title: "Cochera 1"}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-2").Return(entity.SpotDetails{}, errors.NewNotFound("spot", "spot-2"))

	var notified []string
	s.events.EXPECT().Publish(gomock.Any(), booking.RoutingKeyBookingCancelled, gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, _ string, payload any) error {
			event := toJSONMap(s.T(), payload)
			s.Equal(booking.CancelledByOwnerExpired, event["cancelled_by"])
			notified = append(notified, event["driver_user_id"].(string))
			return nil
		},
	)

	result, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(2, result.Expired)
	s.Equal([]string{"b2"}, result.RefundFailures)
	s.Equal([]string{"driver-1", "driver-2"}, notified)
}

func (s *expirePendingApprovalsUseCaseSuite) TestExecute_NothingStale_DoesNothing() {
	s.bookings.EXPECT().ExpireStalePendingApprovals(gomock.Any()).Return(nil, nil)

	result, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(0, result.Expired)
}
