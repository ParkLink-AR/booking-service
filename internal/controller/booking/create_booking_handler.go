package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/create_booking_usecase_mock.go -package=mocks -source=./create_booking_handler.go

// CreateBookingUseCase is the interface consumed by this controller for POST.
type CreateBookingUseCase interface {
	Execute(ctx context.Context, input entity.CreateBookingInput) (entity.Booking, error)
}

func (handler *Handler) CreateBookingEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	var body createBookingRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}

	if _, err := uuid.Parse(body.ParkingSpot.ID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "parking_spot.id must be a valid UUID")
		return
	}
	if _, err := uuid.Parse(body.UserID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "user_id must be a valid UUID")
		return
	}
	start, err := time.Parse(time.RFC3339, body.Start)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TIME_RANGE", "start must be a valid RFC3339 timestamp")
		return
	}
	// For a STAY booking `end` only needs to parse and be after `start` -- Service.CreateBooking
	// overwrites it with parking-service's quoted end date, so mobile can send any placeholder
	// after start (e.g. start + 1h) instead of computing the real end date itself.
	end, err := time.Parse(time.RFC3339, body.End)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TIME_RANGE", "end must be a valid RFC3339 timestamp")
		return
	}
	// Empty Kind means a client built before STAY existed -- default to HOURLY so it keeps
	// working unchanged. Neither kind's Amount is actually trusted from the request body
	// anymore (see Service.CreateBooking, priceStay/priceHourly) -- HOURLY still requires
	// it to be positive here purely as a request-shape check, kept for backwards
	// compatibility with clients still sending their own (now-ignored) estimate.
	kindLabel := body.Kind
	if kindLabel == "" {
		kindLabel = "HOURLY"
	}
	kind, ok := enum.ParseBookingKind(kindLabel)
	if !ok {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_KIND", "kind must be HOURLY or STAY")
		return
	}

	if kind == enum.BookingKindStay {
		if _, err := time.Parse("2006-01-02", body.StayEndDate); err != nil {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_STAY_END_DATE", "stay_end_date must be a YYYY-MM-DD date")
			return
		}
	} else if body.Amount <= 0 {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_AMOUNT", "amount must be a positive integer")
		return
	}

	schedule := make([]entity.ScheduleEntry, len(body.ParkingSpot.Schedule))
	for index, entry := range body.ParkingSpot.Schedule {
		schedule[index] = entity.ScheduleEntry{
			DayOfWeek: entry.DayOfWeek,
			StartHour: entry.StartHour,
			EndHour:   entry.EndHour,
		}
	}

	capacity := body.ParkingSpot.Capacity
	if capacity < 1 {
		capacity = 1
	}

	input := entity.CreateBookingInput{
		ParkingSpotID:       body.ParkingSpot.ID,
		ParkingSpotSchedule: schedule,
		UserID:              body.UserID,
		Start:               start,
		End:                 end,
		Amount:              body.Amount,
		Capacity:            capacity,
		Kind:                kind,
		StayEndDate:         body.StayEndDate,
	}

	created, err := handler.createBookingUseCase.Execute(request.Context(), input)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	// This response always goes straight back to the driver who just created the
	// booking (no code exists yet at this point anyway -- it's generated on CONFIRMED).
	writeJSON(responseWriter, http.StatusCreated, bookingResponseFromEntity(created, handler.location, created.UserID))
}
