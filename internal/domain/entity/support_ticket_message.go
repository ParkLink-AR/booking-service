package entity

import "time"

// SupportTicketMessage is one message in a support ticket's thread. The ticket's opening
// message (its "description") is just the first row here, sent by its creator -- there's no
// separate description field on SupportTicket itself.
type SupportTicketMessage struct {
	ID        string
	TicketID  string
	SenderID  string
	IsAdmin   bool
	Body      string
	CreatedAt time.Time
}
