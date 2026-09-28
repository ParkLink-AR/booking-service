package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_tickets_by_user_repository_mock.go -package=mocks -source=./list_tickets_by_user_usecase.go

// TicketListerByUser reads every ticket a user has ever opened, across every booking.
// Implemented by internal/repository/booking.
type TicketListerByUser interface {
	ListTicketsByUser(ctx context.Context, userID string) ([]entity.TicketSummary, error)
}

// ListTicketsByUserUseCase powers GET /users/{id}/tickets -- self-service, same auth level
// as ListBookingsByUserUseCase (GET /users/{id}/bookings/history). Its only consumer today
// is the mobile app's in-app notification feed.
type ListTicketsByUserUseCase struct {
	tickets TicketListerByUser
}

func NewListTicketsByUserUseCase(tickets TicketListerByUser) *ListTicketsByUserUseCase {
	return &ListTicketsByUserUseCase{tickets: tickets}
}

func (uc *ListTicketsByUserUseCase) Execute(ctx context.Context, userID string) ([]entity.TicketSummary, error) {
	return uc.tickets.ListTicketsByUser(ctx, userID)
}
