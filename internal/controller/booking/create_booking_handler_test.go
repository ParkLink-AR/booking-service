package booking_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

const validUserID = "11111111-1111-1111-1111-111111111111"

// tuesdayScheduleBody is the parking_spot.schedule JSON fragment sent with every test
// booking below (2026-08-18 is a Tuesday, dayOfWeek=1), open 08:00-20:00.
const tuesdayScheduleBody = `[{"day_of_week":1,"start_hour":8,"end_hour":20}]`

func createBookingBody(start, end string, amount int) string {
	return fmt.Sprintf(
		`{"user_id":"%s","start":"%s","end":"%s","amount":%d,"parking_spot":{"id":"%s","schedule":%s}}`,
		validUserID, start, end, amount, validParkingSpotID, tuesdayScheduleBody,
	)
}

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
			ParkingSpotID:       validParkingSpotID,
			ParkingSpotSchedule: []entity.ScheduleEntry{{DayOfWeek: 1, StartHour: 8, EndHour: 20}},
			UserID:              validUserID,
			Start:               start,
			End:                 end,
			Amount:              5000,
			Capacity:            1, // omitted in the body -> defaulted to 1
		}).
		Return(want, nil)

	recorder := s.doPostAsDriver("/bookings", createBookingBody("2026-08-18T10:00:00-03:00", "2026-08-18T11:00:00-03:00", 5000))

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

func (s *handlerSuite) TestCreateBookingEndpoint_PassesCapacityThrough() {
	start, err := time.Parse(time.RFC3339, "2026-08-18T10:00:00-03:00")
	s.Require().NoError(err)
	end, err := time.Parse(time.RFC3339, "2026-08-18T11:00:00-03:00")
	s.Require().NoError(err)

	s.createBookingUseCase.EXPECT().
		Execute(gomock.Any(), entity.CreateBookingInput{
			ParkingSpotID:       validParkingSpotID,
			ParkingSpotSchedule: []entity.ScheduleEntry{{DayOfWeek: 1, StartHour: 8, EndHour: 20}},
			UserID:              validUserID,
			Start:               start,
			End:                 end,
			Amount:              5000,
			Capacity:            4,
		}).
		Return(entity.Booking{ID: "created-id"}, nil)

	body := fmt.Sprintf(
		`{"user_id":"%s","start":"%s","end":"%s","amount":%d,"parking_spot":{"id":"%s","schedule":%s,"capacity":4}}`,
		validUserID, "2026-08-18T10:00:00-03:00", "2026-08-18T11:00:00-03:00", 5000, validParkingSpotID, tuesdayScheduleBody,
	)
	recorder := s.doPostAsDriver("/bookings", body)

	s.Equal(http.StatusCreated, recorder.Code)
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidSpotID() {
	body := `{"user_id":"` + validUserID + `","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":5000,"parking_spot":{"id":"not-a-uuid","schedule":` + tuesdayScheduleBody + `}}`
	recorder := s.doPostAsDriver("/bookings", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_SPOT_ID")
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidBody() {
	recorder := s.doPostAsDriver("/bookings", `not-json`)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_BODY")
}

func (s *handlerSuite) TestCreateBookingEndpoint_MissingCallerIdentity() {
	recorder := s.doPost("/bookings", createBookingBody("2026-08-18T10:00:00-03:00", "2026-08-18T11:00:00-03:00", 5000))

	s.Equal(http.StatusUnauthorized, recorder.Code)
	s.Contains(recorder.Body.String(), "UNAUTHENTICATED")
}

func (s *handlerSuite) TestCreateBookingEndpoint_IgnoresUserIDInBody() {
	otherUser := "99999999-9999-9999-9999-999999999999"
	body := fmt.Sprintf(
		`{"user_id":"%s","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","amount":5000,"parking_spot":{"id":"%s","schedule":%s}}`,
		otherUser, validParkingSpotID, tuesdayScheduleBody,
	)
	s.createBookingUseCase.EXPECT().Execute(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input entity.CreateBookingInput) (entity.Booking, error) {
			s.Equal(validUserID, input.UserID)
			return entity.Booking{ID: validParkingSpotID, UserID: input.UserID}, nil
		})

	recorder := s.doPostAsDriver("/bookings", body)

	s.Equal(http.StatusCreated, recorder.Code)
}

var invalidTimeRangeCases = []struct {
	name string
	body string
}{
	{name: "invalid start", body: createBookingBody("not-a-time", "2026-08-18T11:00:00-03:00", 5000)},
	{name: "invalid end", body: createBookingBody("2026-08-18T10:00:00-03:00", "not-a-time", 5000)},
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidTimeRange() {
	for _, testCase := range invalidTimeRangeCases {
		recorder := s.doPostAsDriver("/bookings", testCase.body)
		s.Equal(http.StatusBadRequest, recorder.Code, testCase.name)
		s.Contains(recorder.Body.String(), "INVALID_TIME_RANGE", testCase.name)
	}
}

func (s *handlerSuite) TestCreateBookingEndpoint_InvalidAmount() {
	recorder := s.doPostAsDriver("/bookings", createBookingBody("2026-08-18T10:00:00-03:00", "2026-08-18T11:00:00-03:00", 0))

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_AMOUNT")
}

func (s *handlerSuite) TestCreateBookingEndpoint_Stay_InvalidEndDate() {
	body := fmt.Sprintf(
		`{"user_id":"%s","start":"2026-08-18T10:00:00-03:00","end":"2026-08-18T11:00:00-03:00","kind":"STAY","stay_end_date":"not-a-date","parking_spot":{"id":"%s"}}`,
		validUserID, validParkingSpotID,
	)

	recorder := s.doPostAsDriver("/bookings", body)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_STAY_END_DATE")
}

func (s *handlerSuite) TestCreateBookingEndpoint_Conflict() {
	s.createBookingUseCase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(entity.Booking{}, errors.NewConflict("requested time range overlaps with an existing booking"))

	recorder := s.doPostAsDriver("/bookings", createBookingBody("2026-08-18T10:00:00-03:00", "2026-08-18T11:00:00-03:00", 5000))

	s.Equal(http.StatusConflict, recorder.Code)
	s.Contains(recorder.Body.String(), "BOOKING_CONFLICT")
}

func (s *handlerSuite) TestCreateBookingEndpoint_OutsidePublishedHours() {
	s.createBookingUseCase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(entity.Booking{}, errors.NewInvalidInput("OUTSIDE_PUBLISHED_HOURS", "requested time is outside the parking spot's published hours"))

	recorder := s.doPostAsDriver("/bookings", createBookingBody("2026-08-18T22:00:00-03:00", "2026-08-18T23:00:00-03:00", 5000))

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "OUTSIDE_PUBLISHED_HOURS")
}
