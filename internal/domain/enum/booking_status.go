package enum

type BookingStatus int

const (
	BookingStatusPendingPayment BookingStatus = iota
	BookingStatusConfirmed
	BookingStatusCompleted
	BookingStatusCancelled
	BookingStatusExpired
)

var bookingStatusStrings = map[BookingStatus]string{
	BookingStatusPendingPayment: "PENDING_PAYMENT",
	BookingStatusConfirmed:      "CONFIRMED",
	BookingStatusCompleted:      "COMPLETED",
	BookingStatusCancelled:      "CANCELLED",
	BookingStatusExpired:        "EXPIRED",
}

var bookingStatusFromString = map[string]BookingStatus{
	"PENDING_PAYMENT": BookingStatusPendingPayment,
	"CONFIRMED":       BookingStatusConfirmed,
	"COMPLETED":       BookingStatusCompleted,
	"CANCELLED":       BookingStatusCancelled,
	"EXPIRED":         BookingStatusExpired,
}

func (s BookingStatus) String() string {
	if str, ok := bookingStatusStrings[s]; ok {
		return str
	}
	return "UNKNOWN"
}

// ParseBookingStatus maps a Postgres booking_status enum label to its Go representation.
func ParseBookingStatus(s string) (BookingStatus, bool) {
	status, ok := bookingStatusFromString[s]
	return status, ok
}
