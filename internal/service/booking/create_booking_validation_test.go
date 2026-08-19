package booking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
)

func TestCreateBookingValidationSuite(t *testing.T) {
	suite.Run(t, new(createBookingValidationSuite))
}

type createBookingValidationSuite struct {
	suite.Suite
}

var isWithinPublishedHoursCases = []struct {
	name             string
	requestedStart   int
	requestedEnd     int
	publishedWindows []entity.TimeWindow
	want             bool
}{
	{
		name:             "fully contained in a published window",
		requestedStart:   10,
		requestedEnd:     11,
		publishedWindows: []entity.TimeWindow{{Start: mustHour(8), End: mustHour(20)}},
		want:             true,
	},
	{
		name:             "exactly matches the published window",
		requestedStart:   8,
		requestedEnd:     20,
		publishedWindows: []entity.TimeWindow{{Start: mustHour(8), End: mustHour(20)}},
		want:             true,
	},
	{
		name:             "starts before the published window",
		requestedStart:   7,
		requestedEnd:     10,
		publishedWindows: []entity.TimeWindow{{Start: mustHour(8), End: mustHour(20)}},
		want:             false,
	},
	{
		name:             "ends after the published window",
		requestedStart:   19,
		requestedEnd:     21,
		publishedWindows: []entity.TimeWindow{{Start: mustHour(8), End: mustHour(20)}},
		want:             false,
	},
	{
		name:             "no published windows at all (closed day)",
		requestedStart:   10,
		requestedEnd:     11,
		publishedWindows: nil,
		want:             false,
	},
}

func mustHour(hour int) time.Time {
	return time.Date(2026, 8, 18, hour, 0, 0, 0, time.UTC)
}

func (s *createBookingValidationSuite) TestIsWithinPublishedHours() {
	for _, testCase := range isWithinPublishedHoursCases {
		availabilityBase := &entity.AvailabilityBase{Days: []entity.DayBase{
			{Date: mustHour(0), Windows: testCase.publishedWindows},
		}}
		got := isWithinPublishedHours(mustHour(testCase.requestedStart), mustHour(testCase.requestedEnd), availabilityBase)
		s.Equal(testCase.want, got, testCase.name)
	}
}

var hasOverlapCases = []struct {
	name           string
	activeBookings []entity.TimeWindow
	requestedStart int
	requestedEnd   int
	want           bool
}{
	{
		name:           "no active bookings",
		activeBookings: nil,
		requestedStart: 10,
		requestedEnd:   11,
		want:           false,
	},
	{
		name:           "overlaps an active booking",
		activeBookings: []entity.TimeWindow{{Start: mustHour(10), End: mustHour(12)}},
		requestedStart: 11,
		requestedEnd:   13,
		want:           true,
	},
	{
		name:           "touches but does not overlap (half-open)",
		activeBookings: []entity.TimeWindow{{Start: mustHour(10), End: mustHour(11)}},
		requestedStart: 11,
		requestedEnd:   12,
		want:           false,
	},
	{
		name:           "fully outside any active booking",
		activeBookings: []entity.TimeWindow{{Start: mustHour(10), End: mustHour(11)}},
		requestedStart: 14,
		requestedEnd:   15,
		want:           false,
	},
}

func (s *createBookingValidationSuite) TestHasOverlap() {
	for _, testCase := range hasOverlapCases {
		got := hasOverlap(testCase.activeBookings, mustHour(testCase.requestedStart), mustHour(testCase.requestedEnd))
		s.Equal(testCase.want, got, testCase.name)
	}
}
