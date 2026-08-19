package availability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

func TestIntervalArithmeticSuite(t *testing.T) {
	suite.Run(t, new(intervalArithmeticSuite))
}

type intervalArithmeticSuite struct {
	suite.Suite
}

func buildTime(hour, minute int) time.Time {
	return time.Date(2026, 8, 18, hour, minute, 0, 0, time.UTC)
}

var subtractIntervalsCases = []struct {
	name              string
	baseWindows       []entity.TimeWindow
	occupiedIntervals []entity.TimeWindow
	want              []entity.TimeWindow
}{
	{
		name:              "no occupied intervals returns base unchanged",
		baseWindows:       []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		occupiedIntervals: nil,
		want:              []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
	},
	{
		name:              "partial booking splits base into two windows",
		baseWindows:       []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		occupiedIntervals: []entity.TimeWindow{{Start: buildTime(10, 0), End: buildTime(11, 0)}},
		want: []entity.TimeWindow{
			{Start: buildTime(8, 0), End: buildTime(10, 0)},
			{Start: buildTime(11, 0), End: buildTime(20, 0)},
		},
	},
	{
		name:              "booking covering full base leaves nothing free",
		baseWindows:       []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		occupiedIntervals: []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		want:              nil,
	},
	{
		name:        "overlapping bookings are merged before subtracting",
		baseWindows: []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		occupiedIntervals: []entity.TimeWindow{
			{Start: buildTime(9, 0), End: buildTime(11, 0)},
			{Start: buildTime(10, 0), End: buildTime(12, 0)},
		},
		want: []entity.TimeWindow{
			{Start: buildTime(8, 0), End: buildTime(9, 0)},
			{Start: buildTime(12, 0), End: buildTime(20, 0)},
		},
	},
	{
		name:              "booking touching the start edge trims from the left",
		baseWindows:       []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		occupiedIntervals: []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(9, 0)}},
		want:              []entity.TimeWindow{{Start: buildTime(9, 0), End: buildTime(20, 0)}},
	},
	{
		name:              "booking touching the end edge trims from the right",
		baseWindows:       []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		occupiedIntervals: []entity.TimeWindow{{Start: buildTime(19, 0), End: buildTime(20, 0)}},
		want:              []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(19, 0)}},
	},
	{
		name:              "booking outside base window does not affect it",
		baseWindows:       []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
		occupiedIntervals: []entity.TimeWindow{{Start: buildTime(21, 0), End: buildTime(22, 0)}},
		want:              []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(20, 0)}},
	},
	{
		name:              "empty base returns no free windows",
		baseWindows:       nil,
		occupiedIntervals: []entity.TimeWindow{{Start: buildTime(10, 0), End: buildTime(11, 0)}},
		want:              nil,
	},
}

func (s *intervalArithmeticSuite) TestSubtractIntervals() {
	for _, testCase := range subtractIntervalsCases {
		got := subtractIntervals(testCase.baseWindows, testCase.occupiedIntervals)
		s.Equal(testCase.want, got, testCase.name)
	}
}

var determineStatusCases = []struct {
	name        string
	baseIsEmpty bool
	freeWindows []entity.TimeWindow
	want        enum.AvailabilityStatus
}{
	{
		name:        "empty base is closed",
		baseIsEmpty: true,
		freeWindows: nil,
		want:        enum.AvailabilityStatusClosed,
	},
	{
		name:        "base with no free windows is full",
		baseIsEmpty: false,
		freeWindows: nil,
		want:        enum.AvailabilityStatusFull,
	},
	{
		name:        "base with at least one free window is available",
		baseIsEmpty: false,
		freeWindows: []entity.TimeWindow{{Start: buildTime(8, 0), End: buildTime(9, 0)}},
		want:        enum.AvailabilityStatusAvailable,
	},
}

func (s *intervalArithmeticSuite) TestDetermineStatus() {
	for _, testCase := range determineStatusCases {
		got := determineStatus(testCase.baseIsEmpty, testCase.freeWindows)
		s.Equal(testCase.want, got, testCase.name)
	}
}

var filterOverlappingCases = []struct {
	name      string
	intervals []entity.TimeWindow
	from, to  time.Time
	want      []entity.TimeWindow
}{
	{
		name:      "interval fully inside range is kept",
		intervals: []entity.TimeWindow{{Start: buildTime(10, 0), End: buildTime(11, 0)}},
		from:      buildTime(8, 0),
		to:        buildTime(20, 0),
		want:      []entity.TimeWindow{{Start: buildTime(10, 0), End: buildTime(11, 0)}},
	},
	{
		name:      "interval fully outside range is dropped",
		intervals: []entity.TimeWindow{{Start: buildTime(21, 0), End: buildTime(22, 0)}},
		from:      buildTime(8, 0),
		to:        buildTime(20, 0),
		want:      nil,
	},
	{
		name:      "interval touching range end is dropped (half-open)",
		intervals: []entity.TimeWindow{{Start: buildTime(20, 0), End: buildTime(21, 0)}},
		from:      buildTime(8, 0),
		to:        buildTime(20, 0),
		want:      nil,
	},
}

func (s *intervalArithmeticSuite) TestFilterOverlapping() {
	for _, testCase := range filterOverlappingCases {
		got := filterOverlapping(testCase.intervals, testCase.from, testCase.to)
		s.Equal(testCase.want, got, testCase.name)
	}
}

func (s *intervalArithmeticSuite) TestConvertWindowsToUTC() {
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)

	localTime := time.Date(2026, 8, 18, 8, 0, 0, 0, location)
	windows := []entity.TimeWindow{{Start: localTime, End: localTime.Add(time.Hour)}}

	got := convertWindowsToUTC(windows)

	s.Require().Len(got, 1)
	s.True(got[0].Start.Equal(localTime))
	s.Equal(time.UTC, got[0].Start.Location())
}
