package enum

type ReviewTargetType int

const (
	ReviewTargetSpot ReviewTargetType = iota
	ReviewTargetOwner
	ReviewTargetDriver
)

var reviewTargetTypeStrings = map[ReviewTargetType]string{
	ReviewTargetSpot:   "spot",
	ReviewTargetOwner:  "owner",
	ReviewTargetDriver: "driver",
}

var reviewTargetTypeFromString = map[string]ReviewTargetType{
	"spot":   ReviewTargetSpot,
	"owner":  ReviewTargetOwner,
	"driver": ReviewTargetDriver,
}

func (t ReviewTargetType) String() string {
	if str, ok := reviewTargetTypeStrings[t]; ok {
		return str
	}
	return "unknown"
}

// ParseReviewTargetType maps a wire/DB value ("spot", "owner", "driver") to its Go
// representation.
func ParseReviewTargetType(s string) (ReviewTargetType, bool) {
	target, ok := reviewTargetTypeFromString[s]
	return target, ok
}
