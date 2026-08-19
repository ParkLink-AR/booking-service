package enum

type AvailabilityStatus int

const (
	AvailabilityStatusClosed AvailabilityStatus = iota
	AvailabilityStatusFull
	AvailabilityStatusAvailable
)

var availabilityStatusStrings = map[AvailabilityStatus]string{
	AvailabilityStatusClosed:    "closed",
	AvailabilityStatusFull:      "full",
	AvailabilityStatusAvailable: "available",
}

func (s AvailabilityStatus) String() string {
	if str, ok := availabilityStatusStrings[s]; ok {
		return str
	}
	return "unknown"
}
