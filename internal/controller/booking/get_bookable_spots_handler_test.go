package booking_test

import (
	"net/http"

	"github.com/goccy/go-json"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

const otherParkingSpotID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func (s *handlerSuite) TestGetBookableSpotsEndpoint_Success() {
	s.getBookableSpotsUseCase.EXPECT().
		Execute(gomock.Any(), []entity.SpotCapacity{
			{ID: validParkingSpotID, Capacity: 1},
			{ID: otherParkingSpotID, Capacity: 3},
		}, gomock.Any(), gomock.Any()).
		Return([]string{validParkingSpotID}, nil)

	body := `{"spots":[{"id":"` + validParkingSpotID + `","capacity":1},{"id":"` + otherParkingSpotID + `","capacity":3}],"start":"2026-08-24T10:00:00-03:00","end":"2026-08-24T11:00:00-03:00"}`
	recorder := s.doPost("/parking-spots/bookable", body)

	s.Equal(http.StatusOK, recorder.Code)

	var response struct {
		BookableParkingSpotIDs []string `json:"bookable_parking_spot_ids"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
	s.Equal([]string{validParkingSpotID}, response.BookableParkingSpotIDs)
}

func (s *handlerSuite) TestGetBookableSpotsEndpoint_DefaultsMissingCapacityToOne() {
	s.getBookableSpotsUseCase.EXPECT().
		Execute(gomock.Any(), []entity.SpotCapacity{{ID: validParkingSpotID, Capacity: 1}}, gomock.Any(), gomock.Any()).
		Return([]string{validParkingSpotID}, nil)

	body := `{"spots":[{"id":"` + validParkingSpotID + `"}],"start":"2026-08-24T10:00:00-03:00","end":"2026-08-24T11:00:00-03:00"}`
	recorder := s.doPost("/parking-spots/bookable", body)

	s.Equal(http.StatusOK, recorder.Code)
}

func (s *handlerSuite) TestGetBookableSpotsEndpoint_InvalidBody() {
	recorder := s.doPost("/parking-spots/bookable", `not-json`)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_BODY")
}

func (s *handlerSuite) TestGetBookableSpotsEndpoint_EmptySpots() {
	body := `{"spots":[],"start":"2026-08-24T10:00:00-03:00","end":"2026-08-24T11:00:00-03:00"}`
	recorder := s.doPost("/parking-spots/bookable", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_SPOT_ID")
}

func (s *handlerSuite) TestGetBookableSpotsEndpoint_InvalidSpotID() {
	body := `{"spots":[{"id":"not-a-uuid","capacity":1}],"start":"2026-08-24T10:00:00-03:00","end":"2026-08-24T11:00:00-03:00"}`
	recorder := s.doPost("/parking-spots/bookable", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_SPOT_ID")
}

var bookableInvalidTimeRangeCases = []struct {
	name string
	body string
}{
	{name: "invalid start", body: `{"spots":[{"id":"` + validParkingSpotID + `","capacity":1}],"start":"not-a-time","end":"2026-08-24T11:00:00-03:00"}`},
	{name: "invalid end", body: `{"spots":[{"id":"` + validParkingSpotID + `","capacity":1}],"start":"2026-08-24T10:00:00-03:00","end":"not-a-time"}`},
}

func (s *handlerSuite) TestGetBookableSpotsEndpoint_InvalidTimeRange() {
	for _, testCase := range bookableInvalidTimeRangeCases {
		recorder := s.doPost("/parking-spots/bookable", testCase.body)
		s.Equal(http.StatusBadRequest, recorder.Code, testCase.name)
		s.Contains(recorder.Body.String(), "INVALID_TIME_RANGE", testCase.name)
	}
}

func (s *handlerSuite) TestGetBookableSpotsEndpoint_UsecaseInvalidDateRange() {
	s.getBookableSpotsUseCase.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "start must be before end"))

	body := `{"spots":[{"id":"` + validParkingSpotID + `","capacity":1}],"start":"2026-08-24T11:00:00-03:00","end":"2026-08-24T10:00:00-03:00"}`
	recorder := s.doPost("/parking-spots/bookable", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_DATE_RANGE")
}
