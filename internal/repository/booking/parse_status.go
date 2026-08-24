package booking

import "github.com/example/bookings-service/internal/domain/enum"

// parseBookingStatus is a small helper shared by repository scan functions.
func parseBookingStatus(label string) (enum.BookingStatus, bool) {
	return enum.ParseBookingStatus(label)
}
