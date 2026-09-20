package booking_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestApproveBookingUseCaseSuite(t *testing.T) {
	suite.Run(t, new(approveBookingUseCaseSuite))
}

type approveBookingUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	approver   *mocks.MockBookingApprover
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	useCase    *booking.ApproveBookingUseCase
	context    context.Context

	bookingID string
	spotID    string
	driverID  string
	ownerID   string
}

func (s *approveBookingUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.approver = mocks.NewMockBookingApprover(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.useCase = booking.NewApproveBookingUseCase(s.approver, s.spots, s.events)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.spotID = "11111111-1111-1111-1111-111111111111"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *approveBookingUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *approveBookingUseCaseSuite) pendingBooking() entity.Booking {
	return entity.Booking{
		ID:            s.bookingID,
		ParkingSpotID: s.spotID,
		UserID:        s.driverID,
		Status:        enum.BookingStatusPendingOwnerApproval,
	}
}

func (s *approveBookingUseCaseSuite) TestExecute_OwnerApproves_ConfirmsAndPublishesToDriver() {
	approved := s.pendingBooking()
	approved.Status = enum.BookingStatusConfirmed

	s.approver.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "Cochera en Palermo"}, nil)
	s.approver.EXPECT().ApproveBooking(gomock.Any(), s.bookingID, gomock.Any()).Return(approved, nil)

	var gotKey string
	var gotPayload map[string]any
	s.events.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, routingKey string, payload any) error {
			gotKey = routingKey
			raw, err := json.Marshal(payload)
			s.Require().NoError(err)
			s.Require().NoError(json.Unmarshal(raw, &gotPayload))
			return nil
		})

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID)

	s.Require().NoError(err)
	s.Equal(approved, got)
	s.Equal("booking.confirmed", gotKey)
	s.Equal("booking.confirmed", gotPayload["event_type"])
	s.Equal(s.driverID, gotPayload["driver_user_id"])
	s.Equal(s.ownerID, gotPayload["owner_user_id"])
	s.Equal("Cochera en Palermo", gotPayload["parking_spot_title"])
}

func (s *approveBookingUseCaseSuite) TestExecute_CallerIsNotOwner_ForbiddenNoApproveNoPublish() {
	s.approver.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "x"}, nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, "44444444-4444-4444-4444-444444444444")

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsForbidden(err))
}

func (s *approveBookingUseCaseSuite) TestExecute_ApproveError_NoPublish() {
	s.approver.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.pendingBooking(), nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), s.spotID).
		Return(entity.SpotDetails{OwnerID: s.ownerID, Title: "x"}, nil)
	s.approver.EXPECT().ApproveBooking(gomock.Any(), s.bookingID, gomock.Any()).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	got, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID)

	s.Equal(entity.Booking{}, got)
	s.True(errors.IsNotFound(err))
}
