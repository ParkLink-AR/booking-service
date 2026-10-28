package enum

// UserBookingsOrder is how GET /users/{id}/bookings/history sorts a driver's bookings (its
// ?sort= query parameter): the ones made most recently first (the default), or the ones
// about to happen first -- see the repository's ListBookingsByUser for the exact ordering.
type UserBookingsOrder int

const (
	UserBookingsOrderRecent UserBookingsOrder = iota
	UserBookingsOrderUpcoming
)

var userBookingsOrderFromString = map[string]UserBookingsOrder{
	"recent":   UserBookingsOrderRecent,
	"upcoming": UserBookingsOrderUpcoming,
}

// ParseUserBookingsOrder maps the ?sort= value to its Go representation. An empty value is
// the default order (recent).
func ParseUserBookingsOrder(s string) (UserBookingsOrder, bool) {
	if s == "" {
		return UserBookingsOrderRecent, true
	}
	order, ok := userBookingsOrderFromString[s]
	return order, ok
}
