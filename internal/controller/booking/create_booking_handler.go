package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
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
	end, err := time.Parse(time.RFC3339, body.End)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TIME_RANGE", "end must be a valid RFC3339 timestamp")
		return
	}
	if body.Amount <= 0 {
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

	input := entity.CreateBookingInput{
		ParkingSpotID:       body.ParkingSpot.ID,
		ParkingSpotSchedule: schedule,
		UserID:              body.UserID,
		Start:               start,
		End:                 end,
		Amount:              body.Amount,
	}

	created, err := handler.createBookingUseCase.Execute(request.Context(), input)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusCreated, bookingResponseFromEntity(created, handler.location))
}
