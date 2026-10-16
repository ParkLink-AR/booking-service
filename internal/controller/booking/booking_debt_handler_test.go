package booking_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	bookingcontroller "github.com/example/bookings-service/internal/controller/booking"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestBookingDebtHandlerSuite(t *testing.T) {
	suite.Run(t, new(bookingDebtHandlerSuite))
}

type bookingDebtHandlerSuite struct {
	suite.Suite
	controller *gomock.Controller
	useCase    *mocks.MockBookingDebtUseCase
	router     *chi.Mux
}

const (
	debtHandlerBookingID = "22222222-2222-2222-2222-222222222222"
	debtHandlerUserID    = "33333333-3333-3333-3333-333333333333"
	debtHandlerKey       = "internal-secret"
)

func (s *bookingDebtHandlerSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.useCase = mocks.NewMockBookingDebtUseCase(s.controller)
	s.router = chi.NewRouter()
	bookingcontroller.NewBookingDebtHandler(s.useCase, time.FixedZone("ART", -3*3600), debtHandlerKey).RegisterRoutes(s.router)
}

func (s *bookingDebtHandlerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *bookingDebtHandlerSuite) do(path string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func sampleDebt(status string) entity.BookingDebt {
	start := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	charged := debtHandlerBookingID
	debt := entity.BookingDebt{
		ID: "debt-1", UserID: debtHandlerUserID, SourceBookingID: debtHandlerBookingID, Minutes: 20, Amount: 4000,
		Status: status, CreatedAt: start.Add(3 * time.Hour), SourceStart: start, SourceEnd: start.Add(2 * time.Hour),
		ParkingSpotID: "spot-1", SpotTitle: "Cochera Centro",
	}
	if status == entity.DebtPaid {
		paidAt := start.Add(48 * time.Hour)
		debt.PaidAt = &paidAt
		debt.ChargedBookingID = &charged
	}
	return debt
}

func (s *bookingDebtHandlerSuite) TestMine_ReturnsWhatTheNextBookingWillAddAndEachDebt() {
	s.useCase.EXPECT().Mine(gomock.Any(), debtHandlerUserID).Return(entity.DebtSummary{
		PendingAmount: 4000, Debts: []entity.BookingDebt{sampleDebt(entity.DebtPending)},
	}, nil)

	recorder := s.do("/debts", map[string]string{"X-User-Id": debtHandlerUserID})

	s.Equal(http.StatusOK, recorder.Code)
	var body struct {
		PendingAmount int `json:"pending_amount"`
		Debts         []struct {
			Status           string `json:"status"`
			Amount           int    `json:"amount"`
			Minutes          int    `json:"minutes"`
			BookingStart     string `json:"booking_start"`
			ParkingSpotTitle string `json:"parking_spot_title"`
		} `json:"debts"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&body))
	s.Equal(4000, body.PendingAmount)
	s.Require().Len(body.Debts, 1)
	s.Equal("PENDING", body.Debts[0].Status)
	s.Equal(20, body.Debts[0].Minutes)
	s.Equal("Cochera Centro", body.Debts[0].ParkingSpotTitle)
	s.Equal("2026-10-12T15:00:00-03:00", body.Debts[0].BookingStart, "times come in Buenos Aires time")
}

func (s *bookingDebtHandlerSuite) TestMine_WithoutAUser_BadRequest() {
	recorder := s.do("/debts", nil)

	s.Equal(http.StatusBadRequest, recorder.Code)
}

func (s *bookingDebtHandlerSuite) TestMine_NothingOwed_EmptyListNotNull() {
	s.useCase.EXPECT().Mine(gomock.Any(), debtHandlerUserID).Return(entity.DebtSummary{}, nil)

	recorder := s.do("/debts", map[string]string{"X-User-Id": debtHandlerUserID})

	s.Equal(http.StatusOK, recorder.Code)
	s.JSONEq(`{"pending_amount":0,"debts":[]}`, recorder.Body.String())
}

func (s *bookingDebtHandlerSuite) TestChargedTo_WithTheInternalKey_ListsThePaidOnesWithWhenAndWhere() {
	s.useCase.EXPECT().ChargedTo(gomock.Any(), debtHandlerBookingID).Return([]entity.BookingDebt{sampleDebt(entity.DebtPaid)}, nil)

	recorder := s.do("/internal/bookings/"+debtHandlerBookingID+"/debts", map[string]string{"X-Internal-Key": debtHandlerKey})

	s.Equal(http.StatusOK, recorder.Code)
	var body struct {
		Debts []struct {
			ChargedBookingID string `json:"charged_booking_id"`
			PaidAt           string `json:"paid_at"`
		} `json:"debts"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&body))
	s.Require().Len(body.Debts, 1)
	s.Equal(debtHandlerBookingID, body.Debts[0].ChargedBookingID)
	s.NotEmpty(body.Debts[0].PaidAt)
}

func (s *bookingDebtHandlerSuite) TestChargedTo_WithoutTheInternalKey_Unauthorized() {
	recorder := s.do("/internal/bookings/"+debtHandlerBookingID+"/debts", map[string]string{"X-User-Id": debtHandlerUserID})

	s.Equal(http.StatusUnauthorized, recorder.Code)
}

func (s *bookingDebtHandlerSuite) TestChargedTo_WrongKey_Unauthorized() {
	recorder := s.do("/internal/bookings/"+debtHandlerBookingID+"/debts", map[string]string{"X-Internal-Key": "nope"})

	s.Equal(http.StatusUnauthorized, recorder.Code)
}

func (s *bookingDebtHandlerSuite) TestChargedTo_InvalidBookingID_BadRequest() {
	recorder := s.do("/internal/bookings/not-a-uuid/debts", map[string]string{"X-Internal-Key": debtHandlerKey})

	s.Equal(http.StatusBadRequest, recorder.Code)
}

func (s *bookingDebtHandlerSuite) TestOfBooking_AdminListsBothSides() {
	s.useCase.EXPECT().OfBooking(gomock.Any(), debtHandlerBookingID).Return([]entity.BookingDebt{
		sampleDebt(entity.DebtPending), sampleDebt(entity.DebtPaid),
	}, nil)

	recorder := s.do("/admin/bookings/"+debtHandlerBookingID+"/debts", nil)

	s.Equal(http.StatusOK, recorder.Code)
	var body struct {
		Debts []struct{} `json:"debts"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&body))
	s.Len(body.Debts, 2)
}
