package booking

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_exit_usecase_mock.go -package=mocks -source=./booking_exit_handler.go

// BookingExitUseCase is how a verified hourly booking ends: the driver reads their exit code,
// the owner enters it as the vehicle leaves, and ParkLink support closes by hand what never got
// entered. Implemented by usecase.BookingExitUseCase.
type BookingExitUseCase interface {
	View(ctx context.Context, bookingID string, viewer entity.BookingViewer) (entity.ExitView, error)
	Verify(ctx context.Context, bookingID, callerID, code string) (entity.Booking, entity.BookingExit, error)
	SupportClose(ctx context.Context, bookingID string, exitedAt time.Time) (entity.Booking, entity.BookingExit, error)
}

type BookingExitHandler struct {
	useCase  BookingExitUseCase
	location *time.Location
}

func NewBookingExitHandler(useCase BookingExitUseCase, location *time.Location) *BookingExitHandler {
	return &BookingExitHandler{useCase: useCase, location: location}
}

func (handler *BookingExitHandler) RegisterRoutes(router chi.Router) {
	router.Get("/bookings/{id}/exit", handler.view)
	router.Post("/bookings/{id}/verify-exit", handler.verify)
	router.Get("/admin/bookings/{id}/exit", handler.adminView)
	router.Post("/admin/bookings/{id}/close", handler.adminClose)
}

// exitResponse is a booking's exit as the caller may see it: only the driver gets the code,
// only the driver and ParkLink see what the driver owes, and only the owner and ParkLink what
// the owner is owed.
type exitResponse struct {
	// Awaiting is true while the vehicle's departure still has to be recorded.
	Awaiting bool    `json:"awaiting"`
	ExitCode *string `json:"exit_code,omitempty"`
	ExitedAt *string `json:"exited_at,omitempty"`
	// ClosedBy is OWNER_CODE or SUPPORT once the departure was recorded.
	ClosedBy string `json:"closed_by,omitempty"`
	// How long past the booking's end the vehicle stayed, and how much of it is charged (all of
	// it once it went past the grace period, none before).
	OverstayMinutes int `json:"overstay_minutes"`
	ChargedMinutes  int `json:"charged_minutes"`
	// What the driver owes for the charged minutes, in centavos.
	OverstayAmount *int `json:"overstay_amount,omitempty"`
	// What the spot's owner is owed for them, in centavos.
	OverstayOwnerAmount *int `json:"overstay_owner_amount,omitempty"`
}

func exitResponseFrom(awaiting bool, exit *entity.BookingExit, role entity.ExitRole, location *time.Location) exitResponse {
	response := exitResponse{Awaiting: awaiting}
	if exit == nil {
		return response
	}
	if role == entity.ExitRoleDriver && awaiting {
		code := exit.Code
		response.ExitCode = &code
	}
	if exit.ExitedAt != nil {
		exitedAt := exit.ExitedAt.In(location).Format(time.RFC3339)
		response.ExitedAt = &exitedAt
		response.ClosedBy = exit.ClosedBy
		response.OverstayMinutes = exit.Overstay.Minutes
		response.ChargedMinutes = exit.Overstay.ChargedMinutes
		if role != entity.ExitRoleOwner {
			amount := exit.Overstay.Amount
			response.OverstayAmount = &amount
		}
		if role != entity.ExitRoleDriver {
			ownerAmount := exit.Overstay.OwnerAmount
			response.OverstayOwnerAmount = &ownerAmount
		}
	}
	return response
}

func (handler *BookingExitHandler) view(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	viewerID, ok := requireUser(responseWriter, request)
	if !ok {
		return
	}
	view, err := handler.useCase.View(request.Context(), bookingID, entity.BookingViewer{UserID: viewerID})
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, exitResponseFrom(view.Awaiting, view.Exit, view.Role, handler.location))
}

func (handler *BookingExitHandler) adminView(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	view, err := handler.useCase.View(request.Context(), bookingID, entity.BookingViewer{Privileged: true})
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, exitResponseFrom(view.Awaiting, view.Exit, view.Role, handler.location))
}

type verifyExitRequest struct {
	Code string `json:"code"`
}

func (handler *BookingExitHandler) verify(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	ownerID, ok := requireUser(responseWriter, request)
	if !ok {
		return
	}
	var body verifyExitRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	_, exit, err := handler.useCase.Verify(request.Context(), bookingID, ownerID, body.Code)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, exitResponseFrom(false, &exit, entity.ExitRoleOwner, handler.location))
}

type closeBookingRequest struct {
	// RFC3339: when the vehicle actually left.
	ExitedAt string `json:"exited_at"`
}

func (handler *BookingExitHandler) adminClose(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	var body closeBookingRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	exitedAt, err := time.Parse(time.RFC3339, body.ExitedAt)
	if err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_EXIT_TIME", "exited_at must be an RFC3339 date-time")
		return
	}
	_, exit, err := handler.useCase.SupportClose(request.Context(), bookingID, exitedAt)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, exitResponseFrom(false, &exit, entity.ExitRoleAdmin, handler.location))
}
