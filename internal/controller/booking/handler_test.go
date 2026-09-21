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

	"github.com/example/bookings-service/internal/controller/booking"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerSuite))
}

type handlerSuite struct {
	suite.Suite
	controller                     *gomock.Controller
	listBookingsUseCase            *mocks.MockListBookingsUseCase
	listBookingsByUserUseCase      *mocks.MockListBookingsByUserUseCase
	createBookingUseCase           *mocks.MockCreateBookingUseCase
	confirmBookingUseCase          *mocks.MockConfirmBookingUseCase
	getBookableSpotsUseCase        *mocks.MockGetBookableSpotsUseCase
	approveBookingUseCase          *mocks.MockApproveBookingUseCase
	rejectBookingUseCase           *mocks.MockRejectBookingUseCase
	cancelBookingUseCase           *mocks.MockCancelBookingUseCase
	listBookingsDetailedUseCase    *mocks.MockListBookingsDetailedUseCase
	getBookingUseCase              *mocks.MockGetBookingUseCase
	submitReviewUseCase            *mocks.MockSubmitReviewUseCase
	listReviewsByTargetUseCase     *mocks.MockListReviewsByTargetUseCase
	listReviewsByBookingUseCase    *mocks.MockListReviewsByBookingUseCase
	getSpotRatingsUseCase          *mocks.MockGetSpotRatingsUseCase
	verifyBookingCodeUseCase       *mocks.MockVerifyBookingCodeUseCase
	sendMessageUseCase             *mocks.MockSendMessageUseCase
	listMessagesUseCase            *mocks.MockListMessagesUseCase
	createTicketUseCase            *mocks.MockCreateTicketUseCase
	listMyTicketsUseCase           *mocks.MockListMyTicketsUseCase
	getTicketUseCase               *mocks.MockGetTicketUseCase
	listTicketMessagesUseCase      *mocks.MockListTicketMessagesUseCase
	addTicketMessageUseCase        *mocks.MockAddTicketMessageUseCase
	listAdminTicketsUseCase        *mocks.MockListAdminTicketsUseCase
	getAdminTicketUseCase          *mocks.MockGetAdminTicketUseCase
	listAdminTicketMessagesUseCase *mocks.MockListAdminTicketMessagesUseCase
	adminReplyUseCase              *mocks.MockAdminReplyUseCase
	updateTicketStatusUseCase      *mocks.MockUpdateTicketStatusUseCase
	router                         chi.Router
	location                       *time.Location
}

const validParkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"

func (s *handlerSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.listBookingsUseCase = mocks.NewMockListBookingsUseCase(s.controller)
	s.listBookingsByUserUseCase = mocks.NewMockListBookingsByUserUseCase(s.controller)
	s.createBookingUseCase = mocks.NewMockCreateBookingUseCase(s.controller)
	s.confirmBookingUseCase = mocks.NewMockConfirmBookingUseCase(s.controller)
	s.getBookableSpotsUseCase = mocks.NewMockGetBookableSpotsUseCase(s.controller)
	s.approveBookingUseCase = mocks.NewMockApproveBookingUseCase(s.controller)
	s.rejectBookingUseCase = mocks.NewMockRejectBookingUseCase(s.controller)
	s.cancelBookingUseCase = mocks.NewMockCancelBookingUseCase(s.controller)
	s.listBookingsDetailedUseCase = mocks.NewMockListBookingsDetailedUseCase(s.controller)
	s.getBookingUseCase = mocks.NewMockGetBookingUseCase(s.controller)
	s.submitReviewUseCase = mocks.NewMockSubmitReviewUseCase(s.controller)
	s.listReviewsByTargetUseCase = mocks.NewMockListReviewsByTargetUseCase(s.controller)
	s.listReviewsByBookingUseCase = mocks.NewMockListReviewsByBookingUseCase(s.controller)
	s.getSpotRatingsUseCase = mocks.NewMockGetSpotRatingsUseCase(s.controller)
	s.verifyBookingCodeUseCase = mocks.NewMockVerifyBookingCodeUseCase(s.controller)
	s.sendMessageUseCase = mocks.NewMockSendMessageUseCase(s.controller)
	s.listMessagesUseCase = mocks.NewMockListMessagesUseCase(s.controller)
	s.createTicketUseCase = mocks.NewMockCreateTicketUseCase(s.controller)
	s.listMyTicketsUseCase = mocks.NewMockListMyTicketsUseCase(s.controller)
	s.getTicketUseCase = mocks.NewMockGetTicketUseCase(s.controller)
	s.listTicketMessagesUseCase = mocks.NewMockListTicketMessagesUseCase(s.controller)
	s.addTicketMessageUseCase = mocks.NewMockAddTicketMessageUseCase(s.controller)
	s.listAdminTicketsUseCase = mocks.NewMockListAdminTicketsUseCase(s.controller)
	s.getAdminTicketUseCase = mocks.NewMockGetAdminTicketUseCase(s.controller)
	s.listAdminTicketMessagesUseCase = mocks.NewMockListAdminTicketMessagesUseCase(s.controller)
	s.adminReplyUseCase = mocks.NewMockAdminReplyUseCase(s.controller)
	s.updateTicketStatusUseCase = mocks.NewMockUpdateTicketStatusUseCase(s.controller)

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location

	s.router = chi.NewRouter()
	booking.NewHandler(
		s.listBookingsUseCase, s.listBookingsByUserUseCase, s.createBookingUseCase,
		s.confirmBookingUseCase, s.getBookableSpotsUseCase,
		s.approveBookingUseCase, s.rejectBookingUseCase, s.cancelBookingUseCase,
		s.listBookingsDetailedUseCase,
		s.getBookingUseCase,
		s.submitReviewUseCase, s.listReviewsByTargetUseCase, s.listReviewsByBookingUseCase,
		s.getSpotRatingsUseCase, s.verifyBookingCodeUseCase,
		s.sendMessageUseCase, s.listMessagesUseCase,
		s.createTicketUseCase, s.listMyTicketsUseCase, s.getTicketUseCase,
		s.listTicketMessagesUseCase, s.addTicketMessageUseCase,
		s.listAdminTicketsUseCase, s.getAdminTicketUseCase, s.listAdminTicketMessagesUseCase,
		s.adminReplyUseCase, s.updateTicketStatusUseCase,
		s.location,
	).RegisterRoutes(s.router)
}

func (s *handlerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *handlerSuite) doGet(path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func (s *handlerSuite) doPost(path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func (s *handlerSuite) TestListBookingsEndpoint_SuccessNoFilters() {
	windowStart := time.Date(2026, 8, 18, 10, 0, 0, 0, s.location).UTC()
	windowEnd := time.Date(2026, 8, 18, 11, 0, 0, 0, s.location).UTC()

	days := []entity.DayBookings{
		{
			Date: time.Date(2026, 8, 18, 0, 0, 0, 0, s.location),
			Bookings: []entity.BookingWindow{
				{Start: windowStart, End: windowEnd, Status: enum.BookingStatusConfirmed},
			},
		},
	}
	s.listBookingsUseCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, (*time.Time)(nil), (*time.Time)(nil), (*enum.BookingStatus)(nil), false).
		Return(days, nil)

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings")

	s.Equal(http.StatusOK, recorder.Code)

	var body struct {
		Bookings []struct {
			Date     string `json:"date"`
			Bookings []struct {
				Start  string `json:"start"`
				End    string `json:"end"`
				Status string `json:"status"`
			} `json:"bookings"`
		} `json:"bookings"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&body))
	s.Require().Len(body.Bookings, 1)
	s.Equal("2026-08-18", body.Bookings[0].Date)
	s.Require().Len(body.Bookings[0].Bookings, 1)
	s.Equal("2026-08-18T10:00:00-03:00", body.Bookings[0].Bookings[0].Start)
	s.Equal("2026-08-18T11:00:00-03:00", body.Bookings[0].Bookings[0].End)
	s.Equal("CONFIRMED", body.Bookings[0].Bookings[0].Status)
}

func (s *handlerSuite) TestListBookingsEndpoint_WithRangeAndStatusFilters() {
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, s.location)
	to := time.Date(2026, 8, 20, 0, 0, 0, 0, s.location)
	status := enum.BookingStatusConfirmed

	s.listBookingsUseCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, &from, &to, &status, false).
		Return(nil, nil)

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?from=2026-08-18&to=2026-08-20&status=CONFIRMED")

	s.Equal(http.StatusOK, recorder.Code)
}

func (s *handlerSuite) TestListBookingsEndpoint_InvalidSpotID() {
	recorder := s.doGet("/parking-spots/not-a-uuid/bookings")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_SPOT_ID")
}

func (s *handlerSuite) TestListBookingsEndpoint_InvalidDate() {
	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?from=not-a-date")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_DATE")
}

func (s *handlerSuite) TestListBookingsEndpoint_InvalidStatus() {
	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?status=NOT_A_STATUS")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_STATUS")
}

func (s *handlerSuite) TestListBookingsEndpoint_UsecaseInvalidDateRange() {
	s.listBookingsUseCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "from must be before to"))

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?from=2026-08-20&to=2026-08-18")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_DATE_RANGE")
}
