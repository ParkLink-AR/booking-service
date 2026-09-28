package enum

type SettlementStatus int

const (
	// SettlementStatusPending is a drafted settlement -- an admin has frozen the amount owed
	// (see usecase.DraftSettlementUseCase) but hasn't yet confirmed the bank transfer went
	// out. Its payouts are attached (SettlementID set) but their own Status stays PENDING
	// until confirmation.
	SettlementStatusPending SettlementStatus = iota
	// SettlementStatusPaid means an admin confirmed the transfer actually happened (see
	// usecase.ConfirmSettlementUseCase) -- this is the point settlement.paid is published and
	// the only status the owner's self-service endpoints ever expose.
	SettlementStatusPaid
)

var settlementStatusStrings = map[SettlementStatus]string{
	SettlementStatusPending: "PENDING",
	SettlementStatusPaid:    "PAID",
}

var settlementStatusFromString = map[string]SettlementStatus{
	"PENDING": SettlementStatusPending,
	"PAID":    SettlementStatusPaid,
}

func (s SettlementStatus) String() string {
	if str, ok := settlementStatusStrings[s]; ok {
		return str
	}
	return "UNKNOWN"
}

// ParseSettlementStatus maps a settlements.status column value to its Go representation.
func ParseSettlementStatus(s string) (SettlementStatus, bool) {
	status, ok := settlementStatusFromString[s]
	return status, ok
}
