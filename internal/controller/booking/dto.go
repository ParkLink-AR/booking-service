package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

type scheduleEntryRequest struct {
	DayOfWeek int `json:"day_of_week"`
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
}

type parkingSpotRequest struct {
	ID       string                 `json:"id"`
	Schedule []scheduleEntryRequest `json:"schedule"`
	// Number of identical places the spot rents against this id ("playa"). Omitted or
	// <1 is treated as 1.
	Capacity int `json:"capacity"`
}

type createBookingRequest struct {
	UserID string `json:"user_id"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Amount int    `json:"amount"`
	// Kind is "HOURLY" (default, for backward compatibility with clients built before this
	// field existed) or "STAY". For STAY, StayEndDate is required and End/Amount are
	// ignored -- see CreateBookingEndpoint and Service.CreateBooking. Start's own time
	// (not just its calendar date) matters for STAY too: it's the driver's chosen check-in
	// hour, applied to both ends of the stay.
	Kind        string             `json:"kind,omitempty"`
	StayEndDate string             `json:"stay_end_date,omitempty"`
	ParkingSpot parkingSpotRequest `json:"parking_spot"`
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
	PaymentID     string  `json:"payment_id,omitempty"`
	// PaidAt is set once, the moment ConfirmBooking recorded the Mercado Pago payment --
	// see entity.Booking.PaidAt. Absent (nil) means the booking was never actually paid,
	// regardless of its current status (e.g. EXPIRED, or CANCELLED before ever reaching
	// PENDING_OWNER_APPROVAL).
	PaidAt *string `json:"paid_at,omitempty"`
	Kind   string  `json:"kind"`
	// VerificationCode is only ever populated for the booking's own driver (see
	// bookingResponseFromEntity's viewerID param) -- the owner enters this code after
	// hearing it from the driver in person, so exposing it to the owner through any API
	// response would defeat the whole point of an in-person check.
	VerificationCode *string `json:"verification_code,omitempty"`
	VerifiedAt       *string `json:"verified_at,omitempty"`
	UpdatedAt        string  `json:"updated_at"`
}

// bookingResponseFromEntity maps a booking to the wire format, rendering every timestamp
// as RFC3339 with the given location's offset (e.g. "-03:00"). viewerID is whoever is
// about to receive this response (an X-User-Id header, or the caller's own known identity
// when there's nothing to compare against, e.g. the driver who just created/confirmed the
// booking) -- VerificationCode/VerifiedAt are only included when viewerID matches
// booking.UserID, i.e. when the viewer IS the driver. Pass "" to always omit them (e.g.
// for the owner-facing approve/reject responses).
func bookingResponseFromEntity(booking entity.Booking, location *time.Location, viewerID string) *bookingResponse {
	response := &bookingResponse{
		ID:            booking.ID,
		ParkingSpotID: booking.ParkingSpotID,
		UserID:        booking.UserID,
		Start:         booking.Start.In(location).Format(time.RFC3339),
		End:           booking.End.In(location).Format(time.RFC3339),
		Status:        booking.Status.String(),
		Amount:        booking.Amount,
		PaymentID:     booking.PaymentID,
		Kind:          booking.Kind.String(),
		UpdatedAt:     booking.UpdatedAt.In(location).Format(time.RFC3339),
	}
	if booking.HoldExpiresAt != nil {
		formatted := booking.HoldExpiresAt.In(location).Format(time.RFC3339)
		response.HoldExpiresAt = &formatted
	}
	if booking.PaidAt != nil {
		formatted := booking.PaidAt.In(location).Format(time.RFC3339)
		response.PaidAt = &formatted
	}
	if viewerID != "" && viewerID == booking.UserID {
		response.VerificationCode = booking.VerificationCode
		if booking.VerifiedAt != nil {
			formatted := booking.VerifiedAt.In(location).Format(time.RFC3339)
			response.VerifiedAt = &formatted
		}
	}
	return response
}

type userBookingResponse struct {
	ID            string  `json:"id"`
	ParkingSpotID string  `json:"parking_spot_id"`
	UserID        string  `json:"user_id"`
	Start         string  `json:"start"`
	End           string  `json:"end"`
	Status        string  `json:"status"`
	HoldExpiresAt *string `json:"hold_expires_at,omitempty"`
	Amount        int     `json:"amount"`
	PaymentID     string  `json:"payment_id,omitempty"`
	// See bookingResponse.PaidAt.
	PaidAt *string `json:"paid_at,omitempty"`
	Kind   string  `json:"kind"`
	// Same visibility rule as bookingResponse.VerificationCode -- see
	// ListBookingsByUserEndpoint, which only fills this in when the caller is asking
	// about their own history.
	VerificationCode *string `json:"verification_code,omitempty"`
	VerifiedAt       *string `json:"verified_at,omitempty"`
	UpdatedAt        string  `json:"updated_at"`
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

type reviewResponse struct {
	ID         string `json:"id"`
	BookingID  string `json:"booking_id"`
	ReviewerID string `json:"reviewer_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Rating     int    `json:"rating"`
	Comment    string `json:"comment,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// reviewListResponse is the shape for every "reviews about X" endpoint: the spot's,
// an owner's, or a driver's. AverageRating is 0 when there are no reviews yet.
type reviewListResponse struct {
	AverageRating float64          `json:"average_rating"`
	Count         int              `json:"count"`
	Reviews       []reviewResponse `json:"reviews"`
}

func reviewResponseFromEntity(review entity.Review, location *time.Location) reviewResponse {
	return reviewResponse{
		ID:         review.ID,
		BookingID:  review.BookingID,
		ReviewerID: review.ReviewerID,
		TargetType: review.TargetType.String(),
		TargetID:   review.TargetID,
		Rating:     review.Rating,
		Comment:    review.Comment,
		CreatedAt:  review.CreatedAt.In(location).Format(time.RFC3339),
	}
}

func reviewListResponseFromEntities(reviews []entity.Review, location *time.Location) reviewListResponse {
	responses := make([]reviewResponse, len(reviews))
	sum := 0
	for i, review := range reviews {
		responses[i] = reviewResponseFromEntity(review, location)
		sum += review.Rating
	}
	average := 0.0
	if len(reviews) > 0 {
		average = float64(sum) / float64(len(reviews))
	}
	return reviewListResponse{AverageRating: average, Count: len(reviews), Reviews: responses}
}
