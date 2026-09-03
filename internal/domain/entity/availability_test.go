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
