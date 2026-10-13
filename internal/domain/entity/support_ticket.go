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

// AdminTicket is a support ticket as ParkLink staff triage it: who opened it (the booking's
// driver or the spot's owner), where the conversation stands (who wrote last, and when), and
// the booking it's about -- everything the support list needs without a request per ticket.
type AdminTicket struct {
	SupportTicket
	// When the last message on the thread was sent, and whether it came from staff. Nil when
	// the thread has no messages (a ticket is always created with its first one).
	LastMessageAt        *time.Time
	LastMessageFromStaff bool
	// The booking's driver: the ticket's creator is either them or the spot's owner.
	BookingUserID string
	ParkingSpotID string
	BookingStart  time.Time
	BookingEnd    time.Time
	BookingStatus string
	BookingKind   string
}

// CreatorIsDriver reports whether the ticket was opened by the booking's driver (otherwise
// it was the spot's owner -- the only other side allowed to open one).
func (ticket AdminTicket) CreatorIsDriver() bool {
	return ticket.CreatedBy == ticket.BookingUserID
}

// AwaitingReply reports whether the ticket is waiting on ParkLink: still open or in progress,
// and the user wrote last.
func (ticket AdminTicket) AwaitingReply() bool {
	active := ticket.Status == enum.TicketStatusOpen || ticket.Status == enum.TicketStatusInProgress
	return active && ticket.LastMessageAt != nil && !ticket.LastMessageFromStaff
}
