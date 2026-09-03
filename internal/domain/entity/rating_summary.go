package entity

// RatingSummary is the average rating and review count for one review target
// (a parking spot, an owner, or a driver).
type RatingSummary struct {
	Average float64
	Count   int
}
