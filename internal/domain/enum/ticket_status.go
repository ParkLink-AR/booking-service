package enum

type TicketStatus int

const (
	TicketStatusOpen TicketStatus = iota
	TicketStatusInProgress
	TicketStatusResolved
	TicketStatusClosed
)

var ticketStatusStrings = map[TicketStatus]string{
	TicketStatusOpen:       "OPEN",
	TicketStatusInProgress: "IN_PROGRESS",
	TicketStatusResolved:   "RESOLVED",
	TicketStatusClosed:     "CLOSED",
}

var ticketStatusFromString = map[string]TicketStatus{
	"OPEN":        TicketStatusOpen,
	"IN_PROGRESS": TicketStatusInProgress,
	"RESOLVED":    TicketStatusResolved,
	"CLOSED":      TicketStatusClosed,
}

func (s TicketStatus) String() string {
	if str, ok := ticketStatusStrings[s]; ok {
		return str
	}
	return "UNKNOWN"
}

// ParseTicketStatus maps a support_tickets.status column value to its Go representation.
func ParseTicketStatus(s string) (TicketStatus, bool) {
	status, ok := ticketStatusFromString[s]
	return status, ok
}
