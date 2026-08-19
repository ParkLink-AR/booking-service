package parkingservice

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
)

// MockClient stands in for the real parking-service integration until that service exists.
// It publishes a fixed Monday-to-Friday window in local time, and an empty window on
// weekends. It never fails and never validates that the spot exists.
type MockClient struct {
	location *time.Location
}

func NewMockClient(location *time.Location) *MockClient {
	return &MockClient{location: location}
}

// GetAvailabilityBase returns one DayBase per calendar day in the half-open range [from, to),
// evaluated in the client's local timezone.
func (client *MockClient) GetAvailabilityBase(
	_ context.Context,
	_ string,
	from, to time.Time,
) (*entity.AvailabilityBase, error) {
	var days []entity.DayBase
	for day := dateOnly(from, client.location); day.Before(to); day = day.AddDate(0, 0, 1) {
		days = append(days, entity.DayBase{
			Date:    day,
			Windows: windowsFor(day, client.location),
		})
	}
	return &entity.AvailabilityBase{Days: days}, nil
}

func dateOnly(instant time.Time, location *time.Location) time.Time {
	local := instant.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func windowsFor(day time.Time, location *time.Location) []entity.TimeWindow {
	switch day.Weekday() {
	case time.Saturday, time.Sunday:
		return []entity.TimeWindow{}
	default:
		start := time.Date(day.Year(), day.Month(), day.Day(), constants.MockPublishedOpenHour, 0, 0, 0, location)
		end := time.Date(day.Year(), day.Month(), day.Day(), constants.MockPublishedCloseHour, 0, 0, 0, location)
		return []entity.TimeWindow{{Start: start, End: end}}
	}
}
