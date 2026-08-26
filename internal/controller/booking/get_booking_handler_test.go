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

const validBookingID = "9f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"

func (s *handlerSuite) TestGetBookingEndpoint_Success() {
	start := time.Date(2026, 8, 25, 9, 0, 0, 0, s.location).UTC()
	end := time.Date(2026, 8, 25, 11, 0, 0, 0, s.location).UTC()

	s.getBookingUseCase.EXPECT().
		Execute(gomock.Any(), validBookingID).
		Return(entity.Booking{
			ID:            validBookingID,
			ParkingSpotID: validParkingSpotID,
			UserID:        validUserID,
			Start:         start,
			End:           end,
			Status:        enum.BookingStatusConfirmed,
			Amount:        180000,
		}, nil)

	recorder := s.doGet("/bookings/" + validBookingID)

	s.Equal(http.StatusOK, recorder.Code)

	var response struct {
		ID            string `json:"id"`
		ParkingSpotID string `json:"parking_spot_id"`
		Status        string `json:"status"`
		Amount        int    `json:"amount"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
	s.Equal(validBookingID, response.ID)
	s.Equal(validParkingSpotID, response.ParkingSpotID)
	s.Equal("CONFIRMED", response.Status)
	s.Equal(180000, response.Amount)
}

func (s *handlerSuite) TestGetBookingEndpoint_InvalidBookingID() {
	recorder := s.doGet("/bookings/not-a-uuid")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_BOOKING_ID")
}

func (s *handlerSuite) TestGetBookingEndpoint_NotFound() {
	s.getBookingUseCase.EXPECT().
		Execute(gomock.Any(), validBookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", validBookingID))

	recorder := s.doGet("/bookings/" + validBookingID)

	s.Equal(http.StatusNotFound, recorder.Code)
	s.Contains(recorder.Body.String(), "BOOKING_NOT_FOUND")
}
