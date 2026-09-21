package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/get_admin_ticket_usecase_mock.go -package=mocks -source=./get_admin_ticket_handler.go

// GetAdminTicketUseCase is the interface consumed by this handler for GET /admin/tickets/{id}.
type GetAdminTicketUseCase interface {
	Execute(ctx context.Context, ticketID string) (entity.SupportTicket, entity.Booking, entity.SpotDetails, error)
}

// adminTicketResponse is ticketResponse plus the booking/spot context admin-dashboard needs to
// show the ticket without a second, driver/owner-gated request.
type adminTicketResponse struct {
	ticketResponse
	ParkingSpotTitle string `json:"parking_spot_title"`
	BookingStart     string `json:"booking_start"`
	BookingEnd       string `json:"booking_end"`
}

func (handler *Handler) GetAdminTicketEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	ticketID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(ticketID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_TICKET_ID", "id must be a valid UUID")
		return
	}

	ticket, booking, spot, err := handler.getAdminTicketUseCase.Execute(request.Context(), ticketID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeErrorCode(responseWriter, http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found")
			return
		}
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, adminTicketResponse{
		ticketResponse:   ticketResponseFromEntity(ticket, handler.location),
		ParkingSpotTitle: spot.Title,
		BookingStart:     booking.Start.In(handler.location).Format(time.RFC3339),
		BookingEnd:       booking.End.In(handler.location).Format(time.RFC3339),
	})
}
