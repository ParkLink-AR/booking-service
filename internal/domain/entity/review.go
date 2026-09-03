package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// Review is a 1-5 star rating (with an optional comment) that one side of a
// completed booking leaves for the spot, its owner, or the driver.
type Review struct {
	ID         string
	BookingID  string
	ReviewerID string
	TargetType enum.ReviewTargetType
	TargetID   string
	Rating     int
	Comment    string
	CreatedAt  time.Time
}
