package entity

import "time"

// DemoSpot is a demo parking spot as parking-service lists it for the demo data load.
type DemoSpot struct {
	ID           string
	OwnerID      string
	PricePerHour *float64
}

// DemoReview is one example review of a demo spot.
type DemoReview struct {
	ReviewerID string
	Rating     int
	Comment    string
	CreatedAt  time.Time
}

// DemoSpotSeed is everything loaded for one demo spot: the finished booking its reviews hang
// from (reviews need a booking) and the reviews themselves.
type DemoSpotSeed struct {
	BookingID   string
	Spot        DemoSpot
	DriverID    string
	Start       time.Time
	End         time.Time
	AmountCents int
	Reviews     []DemoReview
}
