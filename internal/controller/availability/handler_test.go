package availability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/controller/availability"
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
	controller *gomock.Controller
	useCase    *mocks.MockGetAvailabilityUseCase
	router     chi.Router
	location   *time.Location
}

const validParkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"

func (s *handlerSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.useCase = mocks.NewMockGetAvailabilityUseCase(s.controller)

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location

	s.router = chi.NewRouter()
	availability.NewHandler(s.useCase, s.location).RegisterRoutes(s.router)
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

func (s *handlerSuite) TestGetAvailabilityEndpoint_Success() {
	windowStart := time.Date(2026, 8, 18, 8, 0, 0, 0, s.location).UTC()
	windowEnd := time.Date(2026, 8, 18, 20, 0, 0, 0, s.location).UTC()

	days := []entity.DayAvailability{
		{
			Date:        time.Date(2026, 8, 18, 0, 0, 0, 0, s.location),
			Status:      enum.AvailabilityStatusAvailable,
			FreeWindows: []entity.TimeWindow{{Start: windowStart, End: windowEnd}},
		},
	}
	s.useCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, time.Date(2026, 8, 18, 0, 0, 0, 0, s.location), time.Date(2026, 8, 19, 0, 0, 0, 0, s.location)).
		Return(days, nil)

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/availability?from=2026-08-18&to=2026-08-19")

	s.Equal(http.StatusOK, recorder.Code)

	var body struct {
		Days []struct {
			Date        string `json:"date"`
			Status      string `json:"status"`
			FreeWindows []struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"free_windows"`
		} `json:"days"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&body))
	s.Require().Len(body.Days, 1)
	s.Equal("2026-08-18", body.Days[0].Date)
	s.Equal("available", body.Days[0].Status)
	s.Require().Len(body.Days[0].FreeWindows, 1)
	s.Equal("2026-08-18T08:00:00-03:00", body.Days[0].FreeWindows[0].Start)
	s.Equal("2026-08-18T20:00:00-03:00", body.Days[0].FreeWindows[0].End)
}

func (s *handlerSuite) TestGetAvailabilityEndpoint_InvalidSpotID() {
	recorder := s.doGet("/parking-spots/not-a-uuid/availability?from=2026-08-18&to=2026-08-19")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_SPOT_ID")
}

var invalidDateCases = []struct {
	name string
	from string
	to   string
}{
	{name: "invalid from", from: "not-a-date", to: "2026-08-19"},
	{name: "invalid to", from: "2026-08-18", to: "not-a-date"},
	{name: "missing from", from: "", to: "2026-08-19"},
}

func (s *handlerSuite) TestGetAvailabilityEndpoint_InvalidDate() {
	for _, testCase := range invalidDateCases {
		recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/availability?from=" + testCase.from + "&to=" + testCase.to)
		s.Equal(http.StatusBadRequest, recorder.Code, testCase.name)
		s.Contains(recorder.Body.String(), "INVALID_DATE", testCase.name)
	}
}

func (s *handlerSuite) TestGetAvailabilityEndpoint_UsecaseInvalidDateRange() {
	s.useCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, gomock.Any(), gomock.Any()).
		Return(nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "from must be before to"))

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/availability?from=2026-08-19&to=2026-08-18")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_DATE_RANGE")
}

func (s *handlerSuite) TestGetAvailabilityEndpoint_UsecaseRangeTooLarge() {
	s.useCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, gomock.Any(), gomock.Any()).
		Return(nil, errors.NewInvalidInput("RANGE_TOO_LARGE", "date range exceeds maximum of 14 days"))

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/availability?from=2026-08-01&to=2026-08-20")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "RANGE_TOO_LARGE")
}

func (s *handlerSuite) TestGetAvailabilityEndpoint_UpstreamError() {
	s.useCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, gomock.Any(), gomock.Any()).
		Return(nil, errors.NewUpstream("availability.GetAvailabilityBase", upstreamFailure))

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/availability?from=2026-08-18&to=2026-08-19")

	s.Equal(http.StatusBadGateway, recorder.Code)
	s.Contains(recorder.Body.String(), "PARKING_SERVICE_ERROR")
}

var upstreamFailure = upstreamFailureFixture{}

type upstreamFailureFixture struct{}

func (upstreamFailureFixture) Error() string { return "connection refused" }
