package booking

import "github.com/example/bookings-service/internal/domain/enum"

// parsePayoutStatus is a small helper shared by repository scan functions.
func parsePayoutStatus(label string) (enum.PayoutStatus, bool) {
	return enum.ParsePayoutStatus(label)
}
