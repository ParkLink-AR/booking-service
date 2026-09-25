package enum

type PayoutStatus int

const (
	PayoutStatusPending PayoutStatus = iota
	PayoutStatusPaid
)

var payoutStatusStrings = map[PayoutStatus]string{
	PayoutStatusPending: "PENDING",
	PayoutStatusPaid:    "PAID",
}

var payoutStatusFromString = map[string]PayoutStatus{
	"PENDING": PayoutStatusPending,
	"PAID":    PayoutStatusPaid,
}

func (s PayoutStatus) String() string {
	if str, ok := payoutStatusStrings[s]; ok {
		return str
	}
	return "UNKNOWN"
}

// ParsePayoutStatus maps a payouts.status column value to its Go representation.
func ParsePayoutStatus(s string) (PayoutStatus, bool) {
	status, ok := payoutStatusFromString[s]
	return status, ok
}
