package entity

import (
	"sort"
	"time"
)

// TimeWindow is a contiguous half-open time interval [Start, End).
type TimeWindow struct {
	Start time.Time
	End   time.Time
}

// MaxConcurrency returns the largest number of windows that overlap the same instant
// anywhere inside [rangeStart, rangeEnd). Each window is first clipped to that range.
// Half-open semantics: a window ending exactly when another starts does not count as
// concurrent (the -1 event is processed before the +1 at the same instant).
//
// This is the rule for a capacity-N spot: a new booking for [rangeStart, rangeEnd) fits
// iff MaxConcurrency(existingActive, rangeStart, rangeEnd) < N.
func MaxConcurrency(windows []TimeWindow, rangeStart, rangeEnd time.Time) int {
	type event struct {
		at    time.Time
		delta int
	}

	events := make([]event, 0, len(windows)*2)
	for _, w := range windows {
		start, end := w.Start, w.End
		if start.Before(rangeStart) {
			start = rangeStart
		}
		if end.After(rangeEnd) {
			end = rangeEnd
		}
		if !start.Before(end) {
			continue
		}
		events = append(events, event{at: start, delta: +1}, event{at: end, delta: -1})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].delta < events[j].delta // -1 before +1 at the same instant
		}
		return events[i].at.Before(events[j].at)
	})

	current, max := 0, 0
	for _, e := range events {
		current += e.delta
		if current > max {
			max = current
		}
	}
	return max
}
