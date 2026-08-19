package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

type createBookingRequest struct {
	UserID string `json:"user_id"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Amount int    `json:"amount"`
}

type bookingResponse struct {
	ID            string  `json:"id"`
	ParkingSpotID string  `json:"parking_spot_id"`
	UserID        string  `json:"user_id"`
	Start         string  `json:"start"`
	End           string  `json:"end"`
	Status        string  `json:"status"`
	HoldExpiresAt *string `json:"hold_expires_at,omitempty"`
	Amount        int     `json:"amount"`
}

// bookingResponseFromEntity maps a created booking to the wire format, rendering every
// timestamp as RFC3339 with the given location's offset (e.g. "-03:00").
func bookingResponseFromEntity(booking entity.Booking, location *time.Location) *bookingResponse {
	response := &bookingResponse{
		ID:            booking.ID,
		ParkingSpotID: booking.ParkingSpotID,
		UserID:        booking.UserID,
		Start:         booking.Start.In(location).Format(time.RFC3339),
		End:           booking.End.In(location).Format(time.RFC3339),
		Status:        booking.Status.String(),
		Amount:        booking.Amount,
	}
	if booking.HoldExpiresAt != nil {
		formatted := booking.HoldExpiresAt.In(location).Format(time.RFC3339)
		response.HoldExpiresAt = &formatted
	}
	return response
}

type bookingWindowResponse struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Status string `json:"status"`
}

type dayBookingsResponse struct {
	Date     string                  `json:"date"`
	Bookings []bookingWindowResponse `json:"bookings"`
}

type listBookingsResponse struct {
	Bookings []dayBookingsResponse `json:"bookings"`
}

// listBookingsResponseFromEntities maps domain days to the wire format, rendering every
// timestamp as RFC3339 with the given location's offset (e.g. "-03:00").
func listBookingsResponseFromEntities(days []entity.DayBookings, location *time.Location) *listBookingsResponse {
	response := &listBookingsResponse{Bookings: make([]dayBookingsResponse, len(days))}
	for dayIndex, day := range days {
		bookings := make([]bookingWindowResponse, len(day.Bookings))
		for bookingIndex, bookingWindow := range day.Bookings {
			bookings[bookingIndex] = bookingWindowResponse{
				Start:  bookingWindow.Start.In(location).Format(time.RFC3339),
				End:    bookingWindow.End.In(location).Format(time.RFC3339),
				Status: bookingWindow.Status.String(),
			}
		}
		response.Bookings[dayIndex] = dayBookingsResponse{
			Date:     day.Date.In(location).Format("2006-01-02"),
			Bookings: bookings,
		}
	}
	return response
}
