package booking

import "github.com/example/bookings-service/internal/domain/enum"

// parseSettlementStatus is a small helper shared by repository scan functions.
func parseSettlementStatus(label string) (enum.SettlementStatus, bool) {
	return enum.ParseSettlementStatus(label)
}
