package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/list_bookings_usecase_mock.go -package=mocks -source=./handler.go

const dateLayout = "2006-01-02"

// ListBookingsUseCase is the interface consumed by this controller for GET.
type ListBookingsUseCase interface {
	Execute(
		ctx context.Context,
		parkingSpotID string,
		from, to *time.Time,
		status *enum.BookingStatus,
		activeOnly bool,
	) ([]entity.DayBookings, error)
}

// ListBookingsByUserUseCase is the interface consumed by GET /users/{id}/bookings/history.
type ListBookingsByUserUseCase interface {
	Execute(ctx context.Context, userID string) ([]entity.Booking, error)
}

// Handler handles HTTP requests for listing and creating a parking spot's bookings, and
// for checking which of a set of spots have no booking in a given time range.
type Handler struct {
	listBookingsUseCase            ListBookingsUseCase
	listBookingsByUserUseCase      ListBookingsByUserUseCase
	createBookingUseCase           CreateBookingUseCase
	confirmBookingUseCase          ConfirmBookingUseCase
	getBookableSpotsUseCase        GetBookableSpotsUseCase
	approveBookingUseCase          ApproveBookingUseCase
	rejectBookingUseCase           RejectBookingUseCase
	cancelBookingUseCase           CancelBookingUseCase
	listBookingsDetailedUseCase    ListBookingsDetailedUseCase
	getBookingUseCase              GetBookingUseCase
	submitReviewUseCase            SubmitReviewUseCase
	listReviewsByTargetUseCase     ListReviewsByTargetUseCase
	listReviewsByBookingUseCase    ListReviewsByBookingUseCase
	getSpotRatingsUseCase          GetSpotRatingsUseCase
	verifyBookingCodeUseCase       VerifyBookingCodeUseCase
	sendMessageUseCase             SendMessageUseCase
	listMessagesUseCase            ListMessagesUseCase
	createTicketUseCase            CreateTicketUseCase
	listMyTicketsUseCase           ListMyTicketsUseCase
	getTicketUseCase               GetTicketUseCase
	listTicketMessagesUseCase      ListTicketMessagesUseCase
	addTicketMessageUseCase        AddTicketMessageUseCase
	listAdminTicketsUseCase        ListAdminTicketsUseCase
	getAdminTicketUseCase          GetAdminTicketUseCase
	listAdminTicketMessagesUseCase ListAdminTicketMessagesUseCase
	adminReplyUseCase              AdminReplyUseCase
	updateTicketStatusUseCase      UpdateTicketStatusUseCase
	location                       *time.Location
}

func NewHandler(
	listBookingsUseCase ListBookingsUseCase,
	listBookingsByUserUseCase ListBookingsByUserUseCase,
	createBookingUseCase CreateBookingUseCase,
	confirmBookingUseCase ConfirmBookingUseCase,
	getBookableSpotsUseCase GetBookableSpotsUseCase,
	approveBookingUseCase ApproveBookingUseCase,
	rejectBookingUseCase RejectBookingUseCase,
	cancelBookingUseCase CancelBookingUseCase,
	listBookingsDetailedUseCase ListBookingsDetailedUseCase,
	getBookingUseCase GetBookingUseCase,
	submitReviewUseCase SubmitReviewUseCase,
	listReviewsByTargetUseCase ListReviewsByTargetUseCase,
	listReviewsByBookingUseCase ListReviewsByBookingUseCase,
	getSpotRatingsUseCase GetSpotRatingsUseCase,
	verifyBookingCodeUseCase VerifyBookingCodeUseCase,
	sendMessageUseCase SendMessageUseCase,
	listMessagesUseCase ListMessagesUseCase,
	createTicketUseCase CreateTicketUseCase,
	listMyTicketsUseCase ListMyTicketsUseCase,
	getTicketUseCase GetTicketUseCase,
	listTicketMessagesUseCase ListTicketMessagesUseCase,
	addTicketMessageUseCase AddTicketMessageUseCase,
	listAdminTicketsUseCase ListAdminTicketsUseCase,
	getAdminTicketUseCase GetAdminTicketUseCase,
	listAdminTicketMessagesUseCase ListAdminTicketMessagesUseCase,
	adminReplyUseCase AdminReplyUseCase,
	updateTicketStatusUseCase UpdateTicketStatusUseCase,
	location *time.Location,
) *Handler {
	return &Handler{
		listBookingsUseCase:            listBookingsUseCase,
		listBookingsByUserUseCase:      listBookingsByUserUseCase,
		createBookingUseCase:           createBookingUseCase,
		confirmBookingUseCase:          confirmBookingUseCase,
		getBookableSpotsUseCase:        getBookableSpotsUseCase,
		approveBookingUseCase:          approveBookingUseCase,
		rejectBookingUseCase:           rejectBookingUseCase,
		cancelBookingUseCase:           cancelBookingUseCase,
		listBookingsDetailedUseCase:    listBookingsDetailedUseCase,
		getBookingUseCase:              getBookingUseCase,
		submitReviewUseCase:            submitReviewUseCase,
		listReviewsByTargetUseCase:     listReviewsByTargetUseCase,
		listReviewsByBookingUseCase:    listReviewsByBookingUseCase,
		getSpotRatingsUseCase:          getSpotRatingsUseCase,
		verifyBookingCodeUseCase:       verifyBookingCodeUseCase,
		sendMessageUseCase:             sendMessageUseCase,
		listMessagesUseCase:            listMessagesUseCase,
		createTicketUseCase:            createTicketUseCase,
		listMyTicketsUseCase:           listMyTicketsUseCase,
		getTicketUseCase:               getTicketUseCase,
		listTicketMessagesUseCase:      listTicketMessagesUseCase,
		addTicketMessageUseCase:        addTicketMessageUseCase,
		listAdminTicketsUseCase:        listAdminTicketsUseCase,
		getAdminTicketUseCase:          getAdminTicketUseCase,
		listAdminTicketMessagesUseCase: listAdminTicketMessagesUseCase,
		adminReplyUseCase:              adminReplyUseCase,
		updateTicketStatusUseCase:      updateTicketStatusUseCase,
		location:                       location,
	}
}

func (handler *Handler) RegisterRoutes(router chi.Router) {
	router.Get(parkingSpotsBasePath+"/{id}/bookings", handler.ListBookingsEndpoint)
	router.Get(parkingSpotsBasePath+"/{id}/bookings/detailed", handler.ListBookingsDetailedEndpoint)
	router.Get(parkingSpotsBasePath+"/{id}/reviews", handler.ListSpotReviewsEndpoint)
	router.Get("/users/{id}/bookings/history", handler.ListBookingsByUserEndpoint)
	router.Get("/users/{id}/reviews", handler.ListUserReviewsEndpoint)
	router.Post("/bookings", handler.CreateBookingEndpoint)
	router.Post(parkingSpotsBasePath+"/bookable", handler.GetBookableSpotsEndpoint)
	router.Post(parkingSpotsBasePath+"/ratings", handler.GetSpotRatingsEndpoint)
	router.Post("/bookings/{id}/confirm", handler.ConfirmBookingEndpoint)
	router.Post("/bookings/{id}/approve", handler.ApproveBookingEndpoint)
	router.Post("/bookings/{id}/reject", handler.RejectBookingEndpoint)
	router.Post("/bookings/{id}/cancel", handler.CancelBookingEndpoint)
	router.Post("/bookings/{id}/verify-code", handler.VerifyBookingCodeEndpoint)
	router.Post("/bookings/{id}/reviews", handler.SubmitReviewEndpoint)
	router.Post("/bookings/{id}/messages", handler.SendMessageEndpoint)
	router.Get("/bookings/{id}", handler.GetBookingEndpoint)
	router.Get("/bookings/{id}/reviews", handler.ListBookingReviewsEndpoint)
	router.Get("/bookings/{id}/messages", handler.ListMessagesEndpoint)
	router.Post("/bookings/{id}/tickets", handler.CreateTicketEndpoint)
	router.Get("/bookings/{id}/tickets", handler.ListMyTicketsEndpoint)
	router.Get("/tickets/{id}", handler.GetTicketEndpoint)
	router.Get("/tickets/{id}/messages", handler.ListTicketMessagesEndpoint)
	router.Post("/tickets/{id}/messages", handler.AddTicketMessageEndpoint)
	router.Get("/admin/tickets", handler.ListAdminTicketsEndpoint)
	router.Get("/admin/tickets/{id}", handler.GetAdminTicketEndpoint)
	router.Get("/admin/tickets/{id}/messages", handler.ListAdminTicketMessagesEndpoint)
	router.Post("/admin/tickets/{id}/messages", handler.AdminReplyEndpoint)
	router.Patch("/admin/tickets/{id}/status", handler.UpdateTicketStatusEndpoint)
}

func (handler *Handler) ListBookingsByUserEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	userID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(userID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_USER_ID", "id must be a valid UUID")
		return
	}

	bookings, err := handler.listBookingsByUserUseCase.Execute(request.Context(), userID)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	// Only include verification codes when the caller is asking about their own history
	// -- {id} is untrusted on its own (this endpoint doesn't require it to match the
	// caller), so compare against X-User-Id rather than assuming {id} is the real driver.
	includeCodes := request.Header.Get("X-User-Id") == userID

	response := make([]userBookingResponse, len(bookings))
	for i, b := range bookings {
		r := userBookingResponse{
			ID:            b.ID,
			ParkingSpotID: b.ParkingSpotID,
			UserID:        b.UserID,
			Start:         b.Start.In(handler.location).Format(time.RFC3339),
			End:           b.End.In(handler.location).Format(time.RFC3339),
			Status:        b.Status.String(),
			Amount:        b.Amount,
			PaymentID:     b.PaymentID,
			Kind:          b.Kind.String(),
			UpdatedAt:     b.UpdatedAt.In(handler.location).Format(time.RFC3339),
		}
		if b.HoldExpiresAt != nil {
			formatted := b.HoldExpiresAt.In(handler.location).Format(time.RFC3339)
			r.HoldExpiresAt = &formatted
		}
		if includeCodes {
			r.VerificationCode = b.VerificationCode
			if b.VerifiedAt != nil {
				formatted := b.VerifiedAt.In(handler.location).Format(time.RFC3339)
				r.VerifiedAt = &formatted
			}
		}
		response[i] = r
	}
	writeJSON(responseWriter, http.StatusOK, response)
}

func (handler *Handler) ListBookingsEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	parkingSpotID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(parkingSpotID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_SPOT_ID", "id must be a valid UUID")
		return
	}

	from, err := parseOptionalDate(request.URL.Query().Get("from"), handler.location)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_DATE", "from must be a valid YYYY-MM-DD date")
		return
	}
	to, err := parseOptionalDate(request.URL.Query().Get("to"), handler.location)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_DATE", "to must be a valid YYYY-MM-DD date")
		return
	}

	status, ok := parseOptionalStatus(request.URL.Query().Get("status"))
	if !ok {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_STATUS",
			"status must be one of PENDING_PAYMENT, CONFIRMED, COMPLETED, CANCELLED, EXPIRED")
		return
	}

	activeOnly := request.URL.Query().Get("active") == "true"

	days, err := handler.listBookingsUseCase.Execute(request.Context(), parkingSpotID, from, to, status, activeOnly)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, listBookingsResponseFromEntities(days, handler.location))
}

// parseOptionalDate returns nil, nil when raw is empty (the query param was omitted).
func parseOptionalDate(raw string, location *time.Location) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(dateLayout, raw, location)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// parseOptionalStatus returns (nil, true) when raw is empty (the query param was omitted),
// and (nil, false) when raw does not match a known booking status.
func parseOptionalStatus(raw string) (*enum.BookingStatus, bool) {
	if raw == "" {
		return nil, true
	}
	status, ok := enum.ParseBookingStatus(raw)
	if !ok {
		return nil, false
	}
	return &status, true
}

// writeDomainError translates domain errors to HTTP status codes and machine-readable codes.
// HTTP knowledge lives only in the controller — no other layer imports net/http.
func writeDomainError(responseWriter http.ResponseWriter, err error) {
	switch {
	case errors.IsInvalidInput(err):
		writeErrorCode(responseWriter, http.StatusBadRequest, errors.InvalidInputCode(err), err.Error())
	case errors.IsConflict(err):
		writeErrorCode(responseWriter, http.StatusConflict, "BOOKING_CONFLICT", err.Error())
	case errors.IsForbidden(err):
		writeErrorCode(responseWriter, http.StatusForbidden, "NOT_SPOT_OWNER", err.Error())
	case errors.IsUpstream(err):
		writeErrorCode(responseWriter, http.StatusBadGateway, "PARKING_SERVICE_ERROR", err.Error())
	default:
		writeErrorCode(responseWriter, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
