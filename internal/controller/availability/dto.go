package availability

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

type timeWindowResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type dayAvailabilityResponse struct {
	Date        string               `json:"date"`
	Status      string               `json:"status"`
	FreeWindows []timeWindowResponse `json:"free_windows"`
}

type availabilityResponse struct {
	Days []dayAvailabilityResponse `json:"days"`
}

// availabilityResponseFromEntities maps domain days to the wire format, rendering every
// timestamp as RFC3339 with the given location's offset (e.g. "-03:00").
func availabilityResponseFromEntities(days []entity.DayAvailability, location *time.Location) *availabilityResponse {
	response := &availabilityResponse{Days: make([]dayAvailabilityResponse, len(days))}
	for dayIndex, day := range days {
		windows := make([]timeWindowResponse, len(day.FreeWindows))
		for windowIndex, window := range day.FreeWindows {
			windows[windowIndex] = timeWindowResponse{
				Start: window.Start.In(location).Format(time.RFC3339),
				End:   window.End.In(location).Format(time.RFC3339),
			}
		}
		response.Days[dayIndex] = dayAvailabilityResponse{
			Date:        day.Date.In(location).Format("2006-01-02"),
			Status:      day.Status.String(),
			FreeWindows: windows,
		}
	}
	return response
}
