package entity

// SpotDetails is the subset of a parking spot that booking-service needs but does not
// own: who owns it and its human-facing title. Resolved from parking-service (see
// internal/repository/parkingclient).
type SpotDetails struct {
	OwnerID string
	Title   string
}
