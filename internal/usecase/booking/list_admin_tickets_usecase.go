package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_tickets_repository_mock.go -package=mocks -source=./list_admin_tickets_usecase.go

// AllTicketLister reads every support ticket across every booking, optionally filtered by
// status. Implemented by internal/repository/booking.
type AllTicketLister interface {
	ListAllTickets(ctx context.Context, status *enum.TicketStatus) ([]entity.AdminTicket, error)
}

// ListAdminTicketsUseCase powers GET /admin/tickets. There's no per-caller ownership check
// here -- the gateway only lets this route through for a caller whose JWT carries role:"admin",
// so every ticket is fair game.
type ListAdminTicketsUseCase struct {
	tickets AllTicketLister
}

func NewListAdminTicketsUseCase(tickets AllTicketLister) *ListAdminTicketsUseCase {
	return &ListAdminTicketsUseCase{tickets: tickets}
}

func (uc *ListAdminTicketsUseCase) Execute(ctx context.Context, status *enum.TicketStatus) ([]entity.AdminTicket, error) {
	return uc.tickets.ListAllTickets(ctx, status)
}
