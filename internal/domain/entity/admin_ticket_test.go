package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

func adminTicket(status enum.TicketStatus, lastFromStaff bool) entity.AdminTicket {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	return entity.AdminTicket{
		SupportTicket:        entity.SupportTicket{CreatedBy: "driver-1", Status: status},
		LastMessageAt:        &at,
		LastMessageFromStaff: lastFromStaff,
		BookingUserID:        "driver-1",
	}
}

func TestAdminTicket_AwaitingReply(t *testing.T) {
	assert.True(t, adminTicket(enum.TicketStatusOpen, false).AwaitingReply(), "the user wrote last on an open ticket")
	assert.True(t, adminTicket(enum.TicketStatusInProgress, false).AwaitingReply())
	assert.False(t, adminTicket(enum.TicketStatusInProgress, true).AwaitingReply(), "staff already answered")
	assert.False(t, adminTicket(enum.TicketStatusResolved, false).AwaitingReply(), "a resolved ticket waits on nobody")
	assert.False(t, adminTicket(enum.TicketStatusClosed, false).AwaitingReply())
}

func TestAdminTicket_CreatorIsDriver(t *testing.T) {
	ticket := adminTicket(enum.TicketStatusOpen, false)
	assert.True(t, ticket.CreatorIsDriver())
	ticket.CreatedBy = "owner-1"
	assert.False(t, ticket.CreatorIsDriver())
}
