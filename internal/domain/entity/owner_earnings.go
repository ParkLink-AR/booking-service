package entity

// OwnerEarnings is what an owner made from their spots, as their own share (never the
// driver's price with ParkLink's commission): what the "Mis finanzas" screen shows. Every
// amount is in centavos.
type OwnerEarnings struct {
	// The last months, oldest first; the last one is the current month.
	Months []MonthEarnings
	// Per spot, over the same months -- only spots that earned something.
	Spots []SpotEarnings
	// Everything earned, ever: finished bookings, and what a late cancellation left them.
	TotalEarned int
	// What ParkLink already transferred (PAID settlements).
	PaidOut int
	// TotalEarned not transferred yet.
	ToCollect int
	// Paid bookings still to happen or under way: what they'll earn once they finish.
	Upcoming      int
	UpcomingCount int
}

// MonthEarnings is one calendar month ("2026-10") of an owner's earnings.
type MonthEarnings struct {
	Month    string
	Earned   int
	Bookings int
}

// SpotEarnings is one spot's share of an owner's earnings.
type SpotEarnings struct {
	ParkingSpotID string
	Earned        int
	Bookings      int
}
