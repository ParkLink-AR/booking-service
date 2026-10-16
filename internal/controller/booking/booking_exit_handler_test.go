package booking_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	bookingcontroller "github.com/example/bookings-service/internal/controller/booking"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestBookingExitHandlerSuite(t *testing.T) {
	suite.Run(t, new(bookingExitHandlerSuite))
}

type bookingExitHandlerSuite struct {
	suite.Suite
	controller *gomock.Controller
	useCase    *mocks.MockBookingExitUseCase
	router     *chi.Mux
	location   *time.Location
}

const (
	exitHandlerBookingID = "22222222-2222-2222-2222-222222222222"
	exitHandlerUserID    = "33333333-3333-3333-3333-333333333333"
)

func (s *bookingExitHandlerSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.useCase = mocks.NewMockBookingExitUseCase(s.controller)
	s.location = time.FixedZone("ART", -3*3600)
	s.router = chi.NewRouter()
	bookingcontroller.NewBookingExitHandler(s.useCase, s.location).RegisterRoutes(s.router)
}

func (s *bookingExitHandlerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *bookingExitHandlerSuite) do(method, path, body string, withUser bool) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if withUser {
		request.Header.Set("X-User-Id", exitHandlerUserID)
	}
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

type exitBody struct {
	Awaiting            bool    `json:"awaiting"`
	ExitCode            *string `json:"exit_code"`
	ExitedAt            *string `json:"exited_at"`
	ClosedBy            string  `json:"closed_by"`
	OverstayMinutes     int     `json:"overstay_minutes"`
	ChargedMinutes      int     `json:"charged_minutes"`
	OverstayAmount      *int    `json:"overstay_amount"`
	OverstayOwnerAmount *int    `json:"overstay_owner_amount"`
}

func (s *bookingExitHandlerSuite) decode(recorder *httptest.ResponseRecorder) exitBody {
	var body exitBody
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&body))
	return body
}

func closedExit() *entity.BookingExit {
	exitedAt := time.Date(2026, 10, 12, 20, 30, 0, 0, time.UTC)
	return &entity.BookingExit{
		BookingID: exitHandlerBookingID, Code: "4821", ExitedAt: &exitedAt, ClosedBy: entity.ExitClosedByOwnerCode,
		Overstay: entity.Overstay{Minutes: 30, ChargedMinutes: 30, Amount: 60000, OwnerAmount: 50000},
	}
}

func (s *bookingExitHandlerSuite) TestView_DriverAwaitingExit_GetsTheCode() {
	s.useCase.EXPECT().View(gomock.Any(), exitHandlerBookingID, entity.BookingViewer{UserID: exitHandlerUserID}).Return(
		entity.ExitView{Role: entity.ExitRoleDriver, Awaiting: true, Exit: &entity.BookingExit{Code: "4821"}}, nil)

	recorder := s.do(http.MethodGet, "/bookings/"+exitHandlerBookingID+"/exit", "", true)

	s.Equal(http.StatusOK, recorder.Code)
	body := s.decode(recorder)
	s.True(body.Awaiting)
	s.Require().NotNil(body.ExitCode)
	s.Equal("4821", *body.ExitCode)
	s.Nil(body.ExitedAt)
}

func (s *bookingExitHandlerSuite) TestView_DriverOfClosedBooking_SeesWhatItCostButNoCode() {
	s.useCase.EXPECT().View(gomock.Any(), exitHandlerBookingID, gomock.Any()).Return(
		entity.ExitView{Role: entity.ExitRoleDriver, Exit: closedExit()}, nil)

	body := s.decode(s.do(http.MethodGet, "/bookings/"+exitHandlerBookingID+"/exit", "", true))

	s.False(body.Awaiting)
	s.Nil(body.ExitCode)
	s.Require().NotNil(body.ExitedAt)
	s.Equal("2026-10-12T17:30:00-03:00", *body.ExitedAt)
	s.Equal(30, body.ChargedMinutes)
	s.Require().NotNil(body.OverstayAmount)
	s.Equal(60000, *body.OverstayAmount)
	s.Nil(body.OverstayOwnerAmount, "what the owner is paid is not for the driver")
}

func (s *bookingExitHandlerSuite) TestView_Owner_NeverGetsTheCodeNorTheDriversAmount() {
	s.useCase.EXPECT().View(gomock.Any(), exitHandlerBookingID, gomock.Any()).Return(
		entity.ExitView{Role: entity.ExitRoleOwner, Awaiting: true, Exit: &entity.BookingExit{Code: "4821"}}, nil)

	awaiting := s.decode(s.do(http.MethodGet, "/bookings/"+exitHandlerBookingID+"/exit", "", true))
	s.True(awaiting.Awaiting)
	s.Nil(awaiting.ExitCode)

	s.useCase.EXPECT().View(gomock.Any(), exitHandlerBookingID, gomock.Any()).Return(
		entity.ExitView{Role: entity.ExitRoleOwner, Exit: closedExit()}, nil)

	closed := s.decode(s.do(http.MethodGet, "/bookings/"+exitHandlerBookingID+"/exit", "", true))
	s.Nil(closed.OverstayAmount)
	s.Require().NotNil(closed.OverstayOwnerAmount)
	s.Equal(50000, *closed.OverstayOwnerAmount)
}

func (s *bookingExitHandlerSuite) TestView_WithoutUser_BadRequest() {
	recorder := s.do(http.MethodGet, "/bookings/"+exitHandlerBookingID+"/exit", "", false)

	s.Equal(http.StatusBadRequest, recorder.Code)
}

func (s *bookingExitHandlerSuite) TestView_NotYoursIsForbidden() {
	s.useCase.EXPECT().View(gomock.Any(), exitHandlerBookingID, gomock.Any()).Return(entity.ExitView{}, errors.NewForbidden("no"))

	recorder := s.do(http.MethodGet, "/bookings/"+exitHandlerBookingID+"/exit", "", true)

	s.Equal(http.StatusForbidden, recorder.Code)
}

func (s *bookingExitHandlerSuite) TestVerify_OwnerEntersTheCode() {
	s.useCase.EXPECT().Verify(gomock.Any(), exitHandlerBookingID, exitHandlerUserID, "4821").Return(entity.Booking{}, *closedExit(), nil)

	recorder := s.do(http.MethodPost, "/bookings/"+exitHandlerBookingID+"/verify-exit", `{"code":"4821"}`, true)

	s.Equal(http.StatusOK, recorder.Code)
	body := s.decode(recorder)
	s.False(body.Awaiting)
	s.Equal(entity.ExitClosedByOwnerCode, body.ClosedBy)
	s.Nil(body.ExitCode)
}

func (s *bookingExitHandlerSuite) TestVerify_WrongCodeIsAnInvalidInputError() {
	s.useCase.EXPECT().Verify(gomock.Any(), exitHandlerBookingID, exitHandlerUserID, "0000").
		Return(entity.Booking{}, entity.BookingExit{}, errors.NewInvalidInput("INVALID_CODE", "código incorrecto"))

	recorder := s.do(http.MethodPost, "/bookings/"+exitHandlerBookingID+"/verify-exit", `{"code":"0000"}`, true)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_CODE")
}

func (s *bookingExitHandlerSuite) TestVerify_BodyThatIsNotJSON_BadRequest() {
	recorder := s.do(http.MethodPost, "/bookings/"+exitHandlerBookingID+"/verify-exit", `not json`, true)

	s.Equal(http.StatusBadRequest, recorder.Code)
}

func (s *bookingExitHandlerSuite) TestAdminClose_RecordsTheGivenTime() {
	exitedAt := time.Date(2026, 10, 12, 20, 30, 0, 0, time.UTC)
	closed := closedExit()
	closed.ClosedBy = entity.ExitClosedBySupport
	s.useCase.EXPECT().SupportClose(gomock.Any(), exitHandlerBookingID, exitedAt).Return(entity.Booking{}, *closed, nil)

	recorder := s.do(http.MethodPost, "/admin/bookings/"+exitHandlerBookingID+"/close", `{"exited_at":"2026-10-12T20:30:00Z"}`, false)

	s.Equal(http.StatusOK, recorder.Code)
	body := s.decode(recorder)
	s.Equal(entity.ExitClosedBySupport, body.ClosedBy)
	s.Require().NotNil(body.OverstayAmount)
	s.Require().NotNil(body.OverstayOwnerAmount)
}

func (s *bookingExitHandlerSuite) TestAdminClose_InvalidTime_BadRequest() {
	recorder := s.do(http.MethodPost, "/admin/bookings/"+exitHandlerBookingID+"/close", `{"exited_at":"ayer"}`, false)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_EXIT_TIME")
}

func (s *bookingExitHandlerSuite) TestAdminView_SeesBothAmounts() {
	s.useCase.EXPECT().View(gomock.Any(), exitHandlerBookingID, entity.BookingViewer{Privileged: true}).Return(
		entity.ExitView{Role: entity.ExitRoleAdmin, Exit: closedExit()}, nil)

	body := s.decode(s.do(http.MethodGet, "/admin/bookings/"+exitHandlerBookingID+"/exit", "", false))

	s.Require().NotNil(body.OverstayAmount)
	s.Require().NotNil(body.OverstayOwnerAmount)
}
