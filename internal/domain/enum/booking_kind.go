package enum

// BookingKind distinguishes an hourly slot booking (the original behaviour,
// validated against the spot's published weekly schedule) from a "stay"
// booking (day/week/month rental, validated and priced against
// parking-service's stay config instead -- see
// internal/service/booking/create_booking.go and
// internal/repository/parkingclient's GetStayQuote).
type BookingKind int

const (
	BookingKindHourly BookingKind = iota
	BookingKindStay
)

var bookingKindStrings = map[BookingKind]string{
	BookingKindHourly: "HOURLY",
	BookingKindStay:   "STAY",
}

var bookingKindFromString = map[string]BookingKind{
	"HOURLY": BookingKindHourly,
	"STAY":   BookingKindStay,
}

func (k BookingKind) String() string {
	if str, ok := bookingKindStrings[k]; ok {
		return str
	}
	return "UNKNOWN"
}

// ParseBookingKind maps a Postgres bookings.kind value (or the wire "kind" field of a
// create-booking request) to its Go representation.
func ParseBookingKind(s string) (BookingKind, bool) {
	kind, ok := bookingKindFromString[s]
	return kind, ok
}
