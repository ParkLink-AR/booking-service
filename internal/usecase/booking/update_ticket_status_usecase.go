package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// UpdateTicketStatusUseCase powers PATCH /admin/tickets/{id}/status, letting ParkLink staff
// mark a ticket RESOLVED, CLOSED, or move it back to any other status by hand.
type UpdateTicketStatusUseCase struct {
	status TicketStatusUpdater
}

func NewUpdateTicketStatusUseCase(status TicketStatusUpdater) *UpdateTicketStatusUseCase {
	return &UpdateTicketStatusUseCase{status: status}
}

func (uc *UpdateTicketStatusUseCase) Execute(ctx context.Context, ticketID, rawStatus string) (entity.SupportTicket, error) {
	status, ok := enum.ParseTicketStatus(rawStatus)
	if !ok {
		return entity.SupportTicket{}, errors.NewInvalidInput("INVALID_STATUS", "status must be one of OPEN, IN_PROGRESS, RESOLVED, CLOSED")
	}
	return uc.status.UpdateTicketStatus(ctx, ticketID, status)
}
