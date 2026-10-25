package enum

// BookingKind distinguishes an hourly slot booking (the original behaviour,
// validated against the spot's published weekly schedule) from a "stay"
// booking (day/week/month rental, validated and priced against
// parking-service's stay config instead -- see
// internal/service/booking/create_booking.go and
// internal/repository/parkingclient's GetStayQuote) and a "half-day" ("media
// estadía": 12 hours at a closed price, priced and validated like a stay, by
// parkingclient's GetHalfDayQuote, but ended like an hourly booking -- with the
// exit code, its overstay charge and its reminders -- see TracksExit).
type BookingKind int

const (
	BookingKindHourly BookingKind = iota
	BookingKindStay
	BookingKindHalfDay
)

var bookingKindStrings = map[BookingKind]string{
	BookingKindHourly:  "HOURLY",
	BookingKindStay:    "STAY",
	BookingKindHalfDay: "HALF_DAY",
}

var bookingKindFromString = map[string]BookingKind{
	"HOURLY":   BookingKindHourly,
	"STAY":     BookingKindStay,
	"HALF_DAY": BookingKindHalfDay,
}

func (k BookingKind) String() string {
	if str, ok := bookingKindStrings[k]; ok {
		return str
	}
	return "UNKNOWN"
}

// TracksExit reports whether a booking of this kind ends with the exit code (and its overstay
// charge) once its arrival was verified: hourly bookings and half-days, which are short and
// fixed enough that staying past them takes the next driver's time. A stay ends on its own.
func (k BookingKind) TracksExit() bool {
	return k == BookingKindHourly || k == BookingKindHalfDay
}

// UsesStayPricing reports whether this kind is priced from the spot's stay rates (per vehicle
// when the spot prices its stays so) instead of its hourly rate.
func (k BookingKind) UsesStayPricing() bool {
	return k == BookingKindStay || k == BookingKindHalfDay
}

// ParseBookingKind maps a Postgres bookings.kind value (or the wire "kind" field of a
// create-booking request) to its Go representation.
func ParseBookingKind(s string) (BookingKind, bool) {
	kind, ok := bookingKindFromString[s]
	return kind, ok
}
