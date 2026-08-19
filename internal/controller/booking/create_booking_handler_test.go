package booking_test

import (
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

const validUserID = "11111111-1111-1111-1111-111111111111"

func (s *handlerSuite) TestCreateBookingEndpoint_Success() {
	// Parsed the same way the handler parses the request body's RFC3339 strings, so the
	// expected entity.CreateBookingInput is comparable by gomock's reflect.DeepEqual —
	// time.Parse's fixed-offset Location differs from time.LoadLocation's, even for the
	// same instant.
	start, err := time.Parse(time.RFC3339, "2026-08-18T10:00:00-03:00")
	s.Require().NoError(err)
	end, err := time.Parse(time.RFC3339, "2026-08-18T11:00:00-03:00")
	s.Require().NoError(err)
	holdExpiresAt := time.Date(2026, 8, 18, 9, 45, 0, 0, s.location)

	want := entity.Booking{
		ID:            "created-id",
		ParkingSpotID: validParkingSpotID,
		UserID:        validUserID,
		Start:         start.UTC(),
		End:           end.UTC(),
		Status:        enum.BookingStatusPendingPayment,
		HoldExpiresAt: &holdExpiresAt,
		Amount:        5000,
	}
	s.createBookingUseCase.EXPECT().
		Execute(gomock.Any(), entity.CreateBookingInput{
			ParkingSpotID: validParkingSpotID,
			UserID:        validUserID,
			Start:         start,
			End:           end,
			Amount:        5000,
		}).
		Return(want, nil)

	body := `{"user_id":"` + validUserID + `","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":5000}`
	recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", body)

	s.Equal(http.StatusCreated, recorder.Code)

	var response struct {
		ID            string `json:"id"`
		ParkingSpotID string `json:"parking_spot_id"`
		UserID        string `json:"user_id"`
		Start         string `json:"start"`
		End           string `json:"end"`
		Status        string `json:"status"`
		HoldExpiresAt string `json:"hold_expires_at"`
		Amount        int    `json:"amount"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
	s.Equal("created-id", response.ID)
	s.Equal(validParkingSpotID, response.ParkingSpotID)
	s.Equal(validUserID, response.UserID)
	s.Equal("2026-08-18T10:00:00-03:00", response.Start)
	s.Equal("2026-08-18T11:00:00-03:00", response.End)
	s.Equal("PENDING_PAYMENT", response.Status)
	s.Equal("2026-08-18T09:45:00-03:00", response.HoldExpiresAt)
	s.Equal(5000, response.Amount)
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidSpotID() {
	body := `{"user_id":"` + validUserID + `","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":5000}`
	recorder := s.doPost("/parking-spots/not-a-uuid/bookings", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_SPOT_ID")
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidBody() {
	recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", `not-json`)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_BODY")
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidUserID() {
	body := `{"user_id":"not-a-uuid","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":5000}`
	recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_USER_ID")
}

var invalidTimeRangeCases = []struct {
	name string
	body string
}{
	{name: "invalid start", body: `{"user_id":"` + validUserID + `","start":"not-a-time","end":"2026-08-18T11:00:00-03:00","amount":5000}`},
	{name: "invalid end", body: `{"user_id":"` + validUserID + `","start":"2026-08-18T10:00:00-03:00","end":"not-a-time","amount":5000}`},
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidTimeRange() {
	for _, testCase := range invalidTimeRangeCases {
		recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", testCase.body)
		s.Equal(http.StatusBadRequest, recorder.Code, testCase.name)
		s.Contains(recorder.Body.String(), "INVALID_TIME_RANGE", testCase.name)
	}
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidAmount() {
	body := `{"user_id":"` + validUserID + `","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":0}`
	recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_AMOUNT")
}

func (s *handlerSuite) TestCreateBookingEndpoint_Conflict() {
	s.createBookingUseCase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking"))

	body := `{"user_id":"` + validUserID + `","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":5000}`
	recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", body)

	s.Equal(http.StatusConflict, recorder.Code)
	s.Contains(recorder.Body.String(), "BOOKING_CONFLICT")
}

func (s *handlerSuite) TestCreateBookingEndpoint_OutsidePublishedHours() {
	s.createBookingUseCase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(entity.Booking{}, errors.NewInvalidInput("OUTSIDE_PUBLISHED_HOURS", "requested time is outside the parking spot's published hours"))

	body := `{"user_id":"` + validUserID + `","start":"2026-08-18T22:00:00-03:00","end":"2026-08-18T23:00:00-03:00","amount":5000}`
	recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "OUTSIDE_PUBLISHED_HOURS")
}

func (s *handlerSuite) TestCreateBookingEndpoint_UpstreamError() {
	s.createBookingUseCase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(entity.Booking{}, errors.NewUpstream("booking.GetAvailabilityBase", assertErr))

	body := `{"user_id":"` + validUserID + `","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":5000}`
	recorder := s.doPost("/parking-spots/"+validParkingSpotID+"/bookings", body)

	s.Equal(http.StatusBadGateway, recorder.Code)
	s.Contains(recorder.Body.String(), "PARKING_SERVICE_ERROR")
}

var assertErr = assertErrFixture{}

type assertErrFixture struct{}

func (assertErrFixture) Error() string { return "connection refused" }
