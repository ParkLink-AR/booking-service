package availability

import (
	"sort"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

// convertWindowsToUTC returns each window's absolute instants under the UTC location.
func convertWindowsToUTC(windows []entity.TimeWindow) []entity.TimeWindow {
	converted := make([]entity.TimeWindow, len(windows))
	for index, window := range windows {
		converted[index] = entity.TimeWindow{Start: window.Start.UTC(), End: window.End.UTC()}
	}
	return converted
}

// filterOverlapping returns the intervals that overlap the half-open range [from, to).
func filterOverlapping(intervals []entity.TimeWindow, from, to time.Time) []entity.TimeWindow {
	var overlapping []entity.TimeWindow
	for _, interval := range intervals {
		if interval.Start.Before(to) && interval.End.After(from) {
			overlapping = append(overlapping, interval)
		}
	}
	return overlapping
}

// subtractIntervals subtracts occupiedIntervals from baseWindows, returning the free
// sub-windows sorted by start. Overlapping occupied intervals are merged before
// subtraction; zero-or-negative-length results are discarded.
func subtractIntervals(baseWindows, occupiedIntervals []entity.TimeWindow) []entity.TimeWindow {
	mergedOccupied := mergeIntervals(occupiedIntervals)

	var freeWindows []entity.TimeWindow
	for _, baseWindow := range baseWindows {
		remainingPieces := []entity.TimeWindow{baseWindow}
		for _, occupiedInterval := range mergedOccupied {
			var nextPieces []entity.TimeWindow
			for _, piece := range remainingPieces {
				nextPieces = append(nextPieces, subtractOne(piece, occupiedInterval)...)
			}
			remainingPieces = nextPieces
		}
		for _, piece := range remainingPieces {
			if piece.End.After(piece.Start) {
				freeWindows = append(freeWindows, piece)
			}
		}
	}

	sort.Slice(freeWindows, func(i, j int) bool { return freeWindows[i].Start.Before(freeWindows[j].Start) })
	return freeWindows
}

// determineStatus classifies a day as closed (no published base), full (base fully
// occupied), or available (at least one free window).
func determineStatus(baseIsEmpty bool, freeWindows []entity.TimeWindow) enum.AvailabilityStatus {
	if baseIsEmpty {
		return enum.AvailabilityStatusClosed
	}
	if len(freeWindows) == 0 {
		return enum.AvailabilityStatusFull
	}
	return enum.AvailabilityStatusAvailable
}

// subtractOne returns window minus occupiedInterval, split into 0, 1, or 2 sub-windows.
func subtractOne(window, occupiedInterval entity.TimeWindow) []entity.TimeWindow {
	if !occupiedInterval.Start.Before(window.End) || !occupiedInterval.End.After(window.Start) {
		return []entity.TimeWindow{window}
	}

	var remaining []entity.TimeWindow
	if occupiedInterval.Start.After(window.Start) {
		remaining = append(remaining, entity.TimeWindow{Start: window.Start, End: occupiedInterval.Start})
	}
	if occupiedInterval.End.Before(window.End) {
		remaining = append(remaining, entity.TimeWindow{Start: occupiedInterval.End, End: window.End})
	}
	return remaining
}

// mergeIntervals sorts by start and merges overlapping or touching intervals.
func mergeIntervals(intervals []entity.TimeWindow) []entity.TimeWindow {
	if len(intervals) == 0 {
		return nil
	}

	sortedIntervals := make([]entity.TimeWindow, len(intervals))
	copy(sortedIntervals, intervals)
	sort.Slice(sortedIntervals, func(i, j int) bool { return sortedIntervals[i].Start.Before(sortedIntervals[j].Start) })

	mergedIntervals := []entity.TimeWindow{sortedIntervals[0]}
	for _, currentInterval := range sortedIntervals[1:] {
		lastInterval := &mergedIntervals[len(mergedIntervals)-1]
		if !currentInterval.Start.After(lastInterval.End) {
			if currentInterval.End.After(lastInterval.End) {
				lastInterval.End = currentInterval.End
			}
			continue
		}
		mergedIntervals = append(mergedIntervals, currentInterval)
	}
	return mergedIntervals
}
