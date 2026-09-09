package booking

import "github.com/example/bookings-service/internal/domain/enum"

// parseBookingKind is a small helper shared by repository scan functions.
func parseBookingKind(label string) (enum.BookingKind, bool) {
	return enum.ParseBookingKind(label)
}
