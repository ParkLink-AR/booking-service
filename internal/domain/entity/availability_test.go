package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/example/bookings-service/internal/domain/entity"
)

func at(hour int) time.Time {
	return time.Date(2026, 8, 18, hour, 0, 0, 0, time.UTC)
}

func win(startHour, endHour int) entity.TimeWindow {
	return entity.TimeWindow{Start: at(startHour), End: at(endHour)}
}

func TestMaxConcurrency(t *testing.T) {
	cases := []struct {
		name       string
		windows    []entity.TimeWindow
		rangeStart int
		rangeEnd   int
		want       int
	}{
		{name: "no windows", windows: nil, rangeStart: 10, rangeEnd: 12, want: 0},
		{name: "single overlapping window", windows: []entity.TimeWindow{win(10, 12)}, rangeStart: 10, rangeEnd: 12, want: 1},
		{
			name:       "sequential windows never concurrent",
			windows:    []entity.TimeWindow{win(10, 11), win(11, 12), win(12, 13)},
			rangeStart: 10, rangeEnd: 13, want: 1,
		},
		{
			name:       "three windows all covering the same instant",
			windows:    []entity.TimeWindow{win(9, 13), win(10, 12), win(11, 14)},
			rangeStart: 10, rangeEnd: 12, want: 3,
		},
		{
			name:       "peak of 2 inside a longer range",
			windows:    []entity.TimeWindow{win(10, 12), win(11, 13), win(14, 15)},
			rangeStart: 8, rangeEnd: 16, want: 2,
		},
		{
			name:       "windows touching at the range edge do not count (half-open)",
			windows:    []entity.TimeWindow{win(8, 10), win(12, 14)},
			rangeStart: 10, rangeEnd: 12, want: 0,
		},
		{
			name:       "window clipped to the range still counts",
			windows:    []entity.TimeWindow{win(8, 20)},
			rangeStart: 10, rangeEnd: 12, want: 1,
		},
	}

	for _, tc := range cases {
		got := entity.MaxConcurrency(tc.windows, at(tc.rangeStart), at(tc.rangeEnd))
		assert.Equal(t, tc.want, got, tc.name)
	}
}

func TestOccupancyWindowPadsBothSidesByGracePeriod(t *testing.T) {
	got := entity.OccupancyWindow(win(10, 11))

	assert.True(t, got.Start.Equal(at(10).Add(-entity.GracePeriod)))
	assert.True(t, got.End.Equal(at(11).Add(entity.GracePeriod)))
}

// A new booking is compared as a plain [start, end) against the padded existing ones, so the
// rule that comes out is "bookings on the same spot must be at least GracePeriod apart".
func TestOccupancyWindowKeepsBookingsAtLeastGracePeriodApart(t *testing.T) {
	existing := []entity.TimeWindow{entity.OccupancyWindow(win(10, 11))}
	minutes := func(hour, minute int) time.Time {
		return time.Date(2026, 8, 18, hour, minute, 0, 0, time.UTC)
	}

	cases := []struct {
		name       string
		start, end time.Time
		conflict   bool
	}{
		{name: "back to back after", start: minutes(11, 0), end: minutes(12, 0), conflict: true},
		{name: "14 minutes after", start: minutes(11, 14), end: minutes(12, 0), conflict: true},
		{name: "exactly the grace period after", start: minutes(11, 15), end: minutes(12, 0), conflict: false},
		{name: "back to back before", start: minutes(9, 0), end: minutes(10, 0), conflict: true},
		{name: "ends 14 minutes before", start: minutes(9, 0), end: minutes(9, 46), conflict: true},
		{name: "ends exactly the grace period before", start: minutes(9, 0), end: minutes(9, 45), conflict: false},
		{name: "far away", start: minutes(13, 0), end: minutes(14, 0), conflict: false},
	}

	for _, tc := range cases {
		got := entity.MaxConcurrency(existing, tc.start, tc.end) >= 1
		assert.Equal(t, tc.conflict, got, tc.name)
	}
}
