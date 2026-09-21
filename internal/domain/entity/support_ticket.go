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
