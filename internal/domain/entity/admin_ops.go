package entity

import "time"

// AdminBookingFilter narrows the admin panel's booking search (GET /admin/bookings). Every
// field is optional; zero values don't filter.
type AdminBookingFilter struct {
	// Query matches a booking id (or its first characters) or a vehicle plate.
	Query         string
	Status        string
	ParkingSpotID string
	// UserID matches the booking's driver.
	UserID string
	// From/To bound the booking's own time range (overlapping it).
	From  *time.Time
	To    *time.Time
	Limit int
	// Offset pages through the results (newest first).
	Offset int
}

// AdminBooking is a booking as ParkLink staff see it: the booking itself plus what only
// they deal with -- what was given back to the driver by hand and why.
type AdminBooking struct {
	Booking
	AdminNote          *string
	VerificationReview *string
}

// AdminReview is a review as the moderation screen lists it, hidden or not.
type AdminReview struct {
	Review
	HiddenAt     *time.Time
	HiddenReason *string
	HiddenBy     *string
}

// Report is something a user flagged from the app -- a spot, a person or a review -- for
// ParkLink staff to look at.
type Report struct {
	ID         string
	ReporterID string
	// "spot", "user" or "review".
	TargetType string
	TargetID   string
	Reason     string
	Details    string
	// "PENDING", "RESOLVED" or "DISMISSED".
	Status     string
	AdminNote  *string
	ResolvedBy *string
	ResolvedAt *time.Time
	CreatedAt  time.Time
}

// DailyBookings is one day of the admin dashboard's chart.
type DailyBookings struct {
	Date string
	// Bookings created that day that got paid.
	Paid int
	// Their total, in centavos.
	Amount int
}

// AdminMetrics is the admin dashboard's summary of bookings and money over a period.
type AdminMetrics struct {
	Days int
	// Paid bookings created in the period, and what they moved (centavos).
	PaidBookings int
	GrossAmount  int
	// ParkLink's share of those (amount - owner_amount), net of refunds given back.
	Commission int
	// Given back to drivers, automatically or by an admin, for bookings in the period.
	Refunded int
	// How many bookings are in each status right now (all time).
	StatusCounts map[string]int
	Daily        []DailyBookings
}
