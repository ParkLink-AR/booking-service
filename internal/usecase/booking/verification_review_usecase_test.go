package booking_test

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestResolveVerificationReviewUseCaseSuite(t *testing.T) {
	suite.Run(t, new(resolveVerificationReviewUseCaseSuite))
}

type resolveVerificationReviewUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	resolver   *mocks.MockVerificationReviewResolver
	refunder   *mocks.MockPaymentRefunder
	useCase    *booking.ResolveVerificationReviewUseCase
	context    context.Context
	bookingID  string
}

func (s *resolveVerificationReviewUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.resolver = mocks.NewMockVerificationReviewResolver(s.controller)
	s.refunder = mocks.NewMockPaymentRefunder(s.controller)
	s.useCase = booking.NewResolveVerificationReviewUseCase(s.resolver, s.refunder, nil, nil)
	s.context = context.Background()
	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
}

func (s *resolveVerificationReviewUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *resolveVerificationReviewUseCaseSuite) resolvedBooking(review enum.VerificationReview) entity.Booking {
	return entity.Booking{ID: s.bookingID, PaymentID: "mp-123", Status: enum.BookingStatusCompleted, VerificationReview: &review}
}

func (s *resolveVerificationReviewUseCaseSuite) TestExecute_Approved_DoesNotRefund() {
	s.resolver.EXPECT().ResolveVerificationReview(gomock.Any(), s.bookingID, enum.VerificationReviewApproved).
		Return(s.resolvedBooking(enum.VerificationReviewApproved), nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, enum.VerificationReviewApproved)

	s.Require().NoError(err)
	s.Equal(enum.VerificationReviewApproved, *got.VerificationReview)
}

func (s *resolveVerificationReviewUseCaseSuite) TestExecute_Refunded_RefundsThePayment() {
	s.resolver.EXPECT().ResolveVerificationReview(gomock.Any(), s.bookingID, enum.VerificationReviewRefunded).
		Return(s.resolvedBooking(enum.VerificationReviewRefunded), nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "mp-123").Return(nil)

	got, err := s.useCase.Execute(s.context, s.bookingID, enum.VerificationReviewRefunded)

	s.Require().NoError(err)
	s.Equal(enum.VerificationReviewRefunded, *got.VerificationReview)
}

func (s *resolveVerificationReviewUseCaseSuite) TestExecute_RefundFails_ReturnsUpstreamError() {
	s.resolver.EXPECT().ResolveVerificationReview(gomock.Any(), s.bookingID, enum.VerificationReviewRefunded).
		Return(s.resolvedBooking(enum.VerificationReviewRefunded), nil)
	s.refunder.EXPECT().RefundPayment(gomock.Any(), "mp-123").
		Return(errors.NewUpstream("paymentclient.RefundPayment", stderrors.New("boom")))

	_, err := s.useCase.Execute(s.context, s.bookingID, enum.VerificationReviewRefunded)

	s.True(errors.IsUpstream(err))
}

func (s *resolveVerificationReviewUseCaseSuite) TestExecute_PendingDecision_Rejected() {
	_, err := s.useCase.Execute(s.context, s.bookingID, enum.VerificationReviewPending)

	s.True(errors.IsInvalidInput(err))
}

func (s *resolveVerificationReviewUseCaseSuite) TestExecute_AlreadyResolved_ReturnsConflict() {
	s.resolver.EXPECT().ResolveVerificationReview(gomock.Any(), s.bookingID, enum.VerificationReviewApproved).
		Return(entity.Booking{}, errors.NewConflict("esta reserva no tiene una revisión pendiente"))

	_, err := s.useCase.Execute(s.context, s.bookingID, enum.VerificationReviewApproved)

	s.True(errors.IsConflict(err))
}
