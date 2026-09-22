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

const validBookingID = "22222222-2222-2222-2222-222222222222"

// TestGetBookingEndpoint_IncludesPaidAtWhenPaid guards the fix for the bug where the
// driver/owner detail screen showed a booking as "paid" (with a date) using updated_at,
// which every transition bumps -- not just a real Mercado Pago confirmation. paid_at must
// come through once it's actually set on the entity.
func (s *handlerSuite) TestGetBookingEndpoint_IncludesPaidAtWhenPaid() {
	paidAt := time.Date(2026, 8, 18, 10, 5, 0, 0, s.location)
	s.getBookingUseCase.EXPECT().Execute(gomock.Any(), validBookingID).Return(entity.Booking{
		ID:        validBookingID,
		Status:    enum.BookingStatusConfirmed,
		PaymentID: "mp-123",
		PaidAt:    &paidAt,
	}, nil)

	recorder := s.doGet("/bookings/" + validBookingID)

	s.Equal(http.StatusOK, recorder.Code)
	var response struct {
		PaidAt *string `json:"paid_at"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
	s.Require().NotNil(response.PaidAt)
	s.Equal("2026-08-18T10:05:00-03:00", *response.PaidAt)
}

// TestGetBookingEndpoint_OmitsPaidAtWhenNeverPaid is the other half of the same guard:
// a booking whose payment was never confirmed (e.g. its PENDING_PAYMENT hold expired)
// must not report a paid_at at all, regardless of status.
func (s *handlerSuite) TestGetBookingEndpoint_OmitsPaidAtWhenNeverPaid() {
	s.getBookingUseCase.EXPECT().Execute(gomock.Any(), validBookingID).Return(entity.Booking{
		ID:     validBookingID,
		Status: enum.BookingStatusExpired,
	}, nil)

	recorder := s.doGet("/bookings/" + validBookingID)

	s.Equal(http.StatusOK, recorder.Code)
	var response struct {
		PaidAt *string `json:"paid_at"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
	s.Nil(response.PaidAt)
}

func (s *handlerSuite) TestGetBookingEndpoint_NotFound() {
	s.getBookingUseCase.EXPECT().Execute(gomock.Any(), validBookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", validBookingID))

	recorder := s.doGet("/bookings/" + validBookingID)

	s.Equal(http.StatusNotFound, recorder.Code)
}
