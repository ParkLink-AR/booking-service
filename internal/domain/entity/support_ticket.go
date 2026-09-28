package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// SupportTicket is a help request the driver or the spot's owner opens against a specific
// booking (e.g. a payment issue, damage report, or a no-show) -- see usecase.CreateTicketUseCase
// for who's allowed to open one and usecase.AdminReplyUseCase for how ParkLink staff respond.
type SupportTicket struct {
	ID        string
	BookingID string
	CreatedBy string
	Subject   string
	Status    enum.TicketStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TicketSummary is a SupportTicket enriched with its most recent staff reply (if any) --
// powers GET /users/{id}/tickets, the self-service "every ticket I opened, across every
// booking" list. Its only consumer today is the mobile app's in-app notification feed,
// which needs a way to surface support.ticket_replied there without a second per-ticket
// round trip to list messages -- LastAdminReplyID changing is what tells it "there's a new
// reply here" (see internal/repository/booking/list_tickets_by_user.go for the query).
type TicketSummary struct {
	SupportTicket
	LastAdminReplyID *string
	LastAdminReplyAt *time.Time
}
