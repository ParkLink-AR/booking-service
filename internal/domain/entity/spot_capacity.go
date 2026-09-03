package entity

// SpotCapacity pairs a parking spot id with how many identical places it rents against
// that id. Used by the bookable-spots check so a "playa" (capacity > 1) stays bookable
// until it is actually full.
type SpotCapacity struct {
	ID       string
	Capacity int
}
