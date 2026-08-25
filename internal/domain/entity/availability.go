package entity

import "time"

// TimeWindow is a contiguous half-open time interval [Start, End).
type TimeWindow struct {
	Start time.Time
	End   time.Time
}
