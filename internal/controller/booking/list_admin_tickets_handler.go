package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_admin_tickets_usecase_mock.go -package=mocks -source=./list_admin_tickets_handler.go

// ListAdminTicketsUseCase is the interface consumed by this handler for GET /admin/tickets.
type ListAdminTicketsUseCase interface {
	Execute(ctx context.Context, status *enum.TicketStatus) ([]entity.AdminTicket, error)
}

func (handler *Handler) ListAdminTicketsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	var status *enum.TicketStatus
	if raw := request.URL.Query().Get("status"); raw != "" {
		parsed, ok := enum.ParseTicketStatus(raw)
		if !ok {
			writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_STATUS",
				"status must be one of OPEN, IN_PROGRESS, RESOLVED, CLOSED")
			return
		}
		status = &parsed
	}

	tickets, err := handler.listAdminTicketsUseCase.Execute(request.Context(), status)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	responses := make([]adminTicketListItem, len(tickets))
	for i, ticket := range tickets {
		responses[i] = adminTicketListItemFromEntity(ticket, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, responses)
}

// adminTicketListItem is a ticket as the support list shows it: the plain ticket, who opened
// it, where the conversation stands, and a summary of the booking it's about.
type adminTicketListItem struct {
	ticketResponse
	// "driver" or "owner".
	CreatorRole string `json:"creator_role"`
	// Omitted when the thread has no messages.
	LastMessageAt *string `json:"last_message_at,omitempty"`
	// "user" or "staff".
	LastMessageFrom string                    `json:"last_message_from"`
	AwaitingReply   bool                      `json:"awaiting_reply"`
	Booking         adminTicketBookingSummary `json:"booking"`
}

type adminTicketBookingSummary struct {
	ParkingSpotID string `json:"parking_spot_id"`
	DriverID      string `json:"driver_id"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Status        string `json:"status"`
	Kind          string `json:"kind"`
}

func adminTicketListItemFromEntity(ticket entity.AdminTicket, location *time.Location) adminTicketListItem {
	response := adminTicketListItem{
		ticketResponse:  ticketResponseFromEntity(ticket.SupportTicket, location),
		CreatorRole:     "owner",
		LastMessageFrom: "user",
		AwaitingReply:   ticket.AwaitingReply(),
		Booking: adminTicketBookingSummary{
			ParkingSpotID: ticket.ParkingSpotID,
			DriverID:      ticket.BookingUserID,
			Start:         ticket.BookingStart.In(location).Format(time.RFC3339),
			End:           ticket.BookingEnd.In(location).Format(time.RFC3339),
			Status:        ticket.BookingStatus,
			Kind:          ticket.BookingKind,
		},
	}
	if ticket.CreatorIsDriver() {
		response.CreatorRole = "driver"
	}
	if ticket.LastMessageFromStaff {
		response.LastMessageFrom = "staff"
	}
	if ticket.LastMessageAt != nil {
		formatted := ticket.LastMessageAt.In(location).Format(time.RFC3339)
		response.LastMessageAt = &formatted
	}
	return response
}
