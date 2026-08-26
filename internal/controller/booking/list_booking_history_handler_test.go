package booking_test

import (
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

func (s *handlerSuite) TestListBookingHistoryEndpoint_Success() {
	start := time.Date(2026, 8, 25, 9, 0, 0, 0, s.location).UTC()
	end := time.Date(2026, 8, 25, 11, 0, 0, 0, s.location).UTC()

	s.listBookingHistoryUseCase.EXPECT().
		Execute(gomock.Any(), validUserID).
		Return([]entity.Booking{
			{
				ID:            validBookingID,
				ParkingSpotID: validParkingSpotID,
				UserID:        validUserID,
				Start:         start,
				End:           end,
				Status:        enum.BookingStatusConfirmed,
				Amount:        180000,
			},
		}, nil)

	recorder := s.doGet("/users/" + validUserID + "/bookings/history")

	s.Equal(http.StatusOK, recorder.Code)

	var response struct {
		Bookings []struct {
			ID            string `json:"id"`
			ParkingSpotID string `json:"parking_spot_id"`
			Status        string `json:"status"`
			Amount        int    `json:"amount"`
		} `json:"bookings"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
	s.Require().Len(response.Bookings, 1)
	s.Equal(validBookingID, response.Bookings[0].ID)
	s.Equal(validParkingSpotID, response.Bookings[0].ParkingSpotID)
	s.Equal("CONFIRMED", response.Bookings[0].Status)
	s.Equal(180000, response.Bookings[0].Amount)
}

func (s *handlerSuite) TestListBookingHistoryEndpoint_Empty() {
	s.listBookingHistoryUseCase.EXPECT().
		Execute(gomock.Any(), validUserID).
		Return(nil, nil)

	recorder := s.doGet("/users/" + validUserID + "/bookings/history")

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), `"bookings":[]`)
}

func (s *handlerSuite) TestListBookingHistoryEndpoint_InvalidUserID() {
	recorder := s.doGet("/users/not-a-uuid/bookings/history")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_USER_ID")
}
