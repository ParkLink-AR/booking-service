package entity

// SpotDetails is the subset of a parking spot that booking-service needs but does not
// own: who owns it, its human-facing title, and whether it auto-confirms bookings.
// Resolved from parking-service (see internal/repository/parkingclient).
type SpotDetails struct {
	OwnerID        string
	Title          string
	InstantBooking bool
}
