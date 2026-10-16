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

func TestBookingDebtUseCaseSuite(t *testing.T) {
	suite.Run(t, new(bookingDebtUseCaseSuite))
}

type bookingDebtUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	debts      *mocks.MockBookingDebtRepository
	spots      *mocks.MockSpotDetailsLookup
	useCase    *booking.BookingDebtUseCase
	context    context.Context
}

const (
	debtDriverID  = "22222222-2222-2222-2222-222222222222"
	debtBookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
)

func (s *bookingDebtUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.debts = mocks.NewMockBookingDebtRepository(s.controller)
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.useCase = booking.NewBookingDebtUseCase(s.debts, s.spots)
	s.context = context.Background()
}

func (s *bookingDebtUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func debtOf(status string, amount int, spotID string) entity.BookingDebt {
	return entity.BookingDebt{
		ID: "debt-" + status, UserID: debtDriverID, SourceBookingID: debtBookingID, Minutes: 20,
		Amount: amount, Status: status, ParkingSpotID: spotID,
		SourceStart: time.Now().Add(-48 * time.Hour), SourceEnd: time.Now().Add(-46 * time.Hour),
	}
}

func (s *bookingDebtUseCaseSuite) TestMine_OnlyThePendingOnesAreWhatTheNextBookingAdds() {
	s.debts.EXPECT().ListDebtsByUser(gomock.Any(), debtDriverID).Return([]entity.BookingDebt{
		debtOf(entity.DebtPending, 6000, "spot-1"),
		debtOf(entity.DebtReserved, 4000, "spot-1"),
		debtOf(entity.DebtPaid, 9000, "spot-2"),
	}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{Title: "Cochera Centro"}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-2").Return(entity.SpotDetails{Title: "Cochera Norte"}, nil)

	summary, err := s.useCase.Mine(s.context, debtDriverID)

	s.Require().NoError(err)
	s.Equal(6000, summary.PendingAmount)
	s.Require().Len(summary.Debts, 3)
	s.Equal("Cochera Centro", summary.Debts[0].SpotTitle)
	s.Equal("Cochera Centro", summary.Debts[1].SpotTitle, "a spot is looked up once however many debts it caused")
	s.Equal("Cochera Norte", summary.Debts[2].SpotTitle)
}

func (s *bookingDebtUseCaseSuite) TestMine_NothingOwed() {
	s.debts.EXPECT().ListDebtsByUser(gomock.Any(), debtDriverID).Return(nil, nil)

	summary, err := s.useCase.Mine(s.context, debtDriverID)

	s.Require().NoError(err)
	s.Zero(summary.PendingAmount)
	s.Empty(summary.Debts)
}

func (s *bookingDebtUseCaseSuite) TestMine_ASpotThatCannotBeResolved_OnlyLeavesItsDebtsUntitled() {
	s.debts.EXPECT().ListDebtsByUser(gomock.Any(), debtDriverID).Return([]entity.BookingDebt{debtOf(entity.DebtPending, 6000, "spot-1")}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{}, errors.NewInternalError("parkingclient", context.DeadlineExceeded))

	summary, err := s.useCase.Mine(s.context, debtDriverID)

	s.Require().NoError(err)
	s.Equal(6000, summary.PendingAmount)
	s.Empty(summary.Debts[0].SpotTitle)
}

func (s *bookingDebtUseCaseSuite) TestMine_RepositoryError_Propagates() {
	s.debts.EXPECT().ListDebtsByUser(gomock.Any(), debtDriverID).Return(nil, errors.NewInternalError("booking.ListDebtsByUser", context.Canceled))

	_, err := s.useCase.Mine(s.context, debtDriverID)

	s.Error(err)
}

func (s *bookingDebtUseCaseSuite) TestChargedTo_NamesTheSpotOfEachDebt() {
	s.debts.EXPECT().ListDebtsChargedTo(gomock.Any(), debtBookingID).Return([]entity.BookingDebt{debtOf(entity.DebtReserved, 6000, "spot-1")}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{Title: "Cochera Centro"}, nil)

	debts, err := s.useCase.ChargedTo(s.context, debtBookingID)

	s.Require().NoError(err)
	s.Require().Len(debts, 1)
	s.Equal("Cochera Centro", debts[0].SpotTitle)
}

func (s *bookingDebtUseCaseSuite) TestOfBooking_ListsBothSidesOfTheBooking() {
	s.debts.EXPECT().ListDebtsOfBooking(gomock.Any(), debtBookingID).Return([]entity.BookingDebt{
		debtOf(entity.DebtPending, 6000, "spot-1"),
		debtOf(entity.DebtPaid, 3000, "spot-1"),
	}, nil)
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), "spot-1").Return(entity.SpotDetails{Title: "Cochera Centro"}, nil)

	debts, err := s.useCase.OfBooking(s.context, debtBookingID)

	s.Require().NoError(err)
	s.Len(debts, 2)
}

func (s *bookingDebtUseCaseSuite) TestChargedTo_RepositoryError_Propagates() {
	s.debts.EXPECT().ListDebtsChargedTo(gomock.Any(), debtBookingID).Return(nil, errors.NewInternalError("booking.ListDebtsChargedTo", context.Canceled))

	_, err := s.useCase.ChargedTo(s.context, debtBookingID)

	s.Error(err)
}

func (s *bookingDebtUseCaseSuite) TestOfBooking_RepositoryError_Propagates() {
	s.debts.EXPECT().ListDebtsOfBooking(gomock.Any(), debtBookingID).Return(nil, errors.NewInternalError("booking.ListDebtsOfBooking", context.Canceled))

	_, err := s.useCase.OfBooking(s.context, debtBookingID)

	s.Error(err)
}
