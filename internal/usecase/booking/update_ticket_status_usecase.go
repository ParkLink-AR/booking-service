package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// UpdateTicketStatusUseCase powers PATCH /admin/tickets/{id}/status, letting ParkLink staff
// mark a ticket RESOLVED, CLOSED, or move it back to any other status by hand.
// When a ticket is RESOLVED or CLOSED, whoever opened it is told (support.ticket_closed).
type UpdateTicketStatusUseCase struct {
	status   TicketStatusUpdater
	bookings TicketBookingGetter
	events   EventPublisher
}

func NewUpdateTicketStatusUseCase(status TicketStatusUpdater, bookings TicketBookingGetter, events EventPublisher) *UpdateTicketStatusUseCase {
	return &UpdateTicketStatusUseCase{status: status, bookings: bookings, events: events}
}

func (uc *UpdateTicketStatusUseCase) Execute(ctx context.Context, ticketID, rawStatus string) (entity.SupportTicket, error) {
	status, ok := enum.ParseTicketStatus(rawStatus)
	if !ok {
		return entity.SupportTicket{}, errors.NewInvalidInput("INVALID_STATUS", "status must be one of OPEN, IN_PROGRESS, RESOLVED, CLOSED")
	}
	ticket, err := uc.status.UpdateTicketStatus(ctx, ticketID, status)
	if err != nil {
		return entity.SupportTicket{}, err
	}
	if uc.events != nil && (status == enum.TicketStatusResolved || status == enum.TicketStatusClosed) {
		emitBookingEvent(ctx, uc.events, RoutingKeySupportTicketClosed,
			newSupportTicketClosedEvent(ticket, ticketCreatorRole(ctx, uc.bookings, ticket)))
	}
	return ticket, nil
}

// ticketCreatorRole is which side of its booking a ticket's creator is on, or "" when the
// booking can't be looked up.
func ticketCreatorRole(ctx context.Context, bookings TicketBookingGetter, ticket entity.SupportTicket) string {
	booking, err := bookings.GetByID(ctx, ticket.BookingID)
	if err != nil {
		return ""
	}
	if booking.UserID == ticket.CreatedBy {
		return RecipientRoleDriver
	}
	return RecipientRoleOwner
}
