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

func TestSubmitReviewUseCaseSuite(t *testing.T) {
	suite.Run(t, new(submitReviewUseCaseSuite))
}

type submitReviewUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	getter     *mocks.MockBookingGetter
	owners     *mocks.MockSpotOwnerLookup
	reviews    *mocks.MockReviewCreator
	useCase    *booking.SubmitReviewUseCase
	context    context.Context
}

const (
	reviewBookingID = "b-1"
	reviewSpotID    = "spot-1"
	reviewDriverID  = "driver-1"
	reviewOwnerID   = "owner-1"
)

func (s *submitReviewUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.getter = mocks.NewMockBookingGetter(s.controller)
	s.owners = mocks.NewMockSpotOwnerLookup(s.controller)
	s.reviews = mocks.NewMockReviewCreator(s.controller)
	s.useCase = booking.NewSubmitReviewUseCase(s.getter, s.owners, s.reviews)
	s.context = context.Background()
}

func (s *submitReviewUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *submitReviewUseCaseSuite) booking(status enum.BookingStatus) entity.Booking {
	return entity.Booking{ID: reviewBookingID, ParkingSpotID: reviewSpotID, UserID: reviewDriverID, Status: status}
}

func (s *submitReviewUseCaseSuite) TestExecute_OwnerRatesDriverOfCompletedBooking() {
	s.getter.EXPECT().GetByID(gomock.Any(), reviewBookingID).Return(s.booking(enum.BookingStatusCompleted), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), reviewSpotID).Return(reviewOwnerID, nil)
	s.reviews.EXPECT().CreateReview(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, review entity.Review) (bool, entity.Review, error) {
			s.Equal(reviewDriverID, review.TargetID)
			return true, review, nil
		})

	_, err := s.useCase.Execute(s.context, reviewBookingID, reviewOwnerID, enum.ReviewTargetDriver, 5, "")

	s.Require().NoError(err)
}

func (s *submitReviewUseCaseSuite) TestExecute_SecondReviewOfSameTargetIsRejected() {
	s.getter.EXPECT().GetByID(gomock.Any(), reviewBookingID).Return(s.booking(enum.BookingStatusCompleted), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), reviewSpotID).Return(reviewOwnerID, nil)
	s.reviews.EXPECT().CreateReview(gomock.Any(), gomock.Any()).Return(false, entity.Review{}, nil)

	_, err := s.useCase.Execute(s.context, reviewBookingID, reviewOwnerID, enum.ReviewTargetDriver, 4, "")

	s.True(errors.IsConflict(err))
}

func (s *submitReviewUseCaseSuite) TestExecute_BookingNotFinishedYetIsRejected() {
	s.getter.EXPECT().GetByID(gomock.Any(), reviewBookingID).Return(s.booking(enum.BookingStatusInProgress), nil)

	_, err := s.useCase.Execute(s.context, reviewBookingID, reviewDriverID, enum.ReviewTargetSpot, 5, "")

	s.True(errors.IsConflict(err))
}

func (s *submitReviewUseCaseSuite) TestExecute_DriverCannotRateThemselves() {
	s.getter.EXPECT().GetByID(gomock.Any(), reviewBookingID).Return(s.booking(enum.BookingStatusCompleted), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), reviewSpotID).Return(reviewOwnerID, nil)

	_, err := s.useCase.Execute(s.context, reviewBookingID, reviewDriverID, enum.ReviewTargetDriver, 5, "")

	s.True(errors.IsForbidden(err))
}

func (s *submitReviewUseCaseSuite) TestExecute_RatingOutOfRangeIsRejected() {
	_, err := s.useCase.Execute(s.context, reviewBookingID, reviewDriverID, enum.ReviewTargetSpot, 6, "")

	s.True(errors.IsInvalidInput(err))
}
