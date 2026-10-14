package booking

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/admin_ops_usecase_mock.go -package=mocks -source=./admin_ops_handler.go

// AdminOpsUseCase is what the admin panel's booking, review, report and dashboard tools
// need. Implemented by usecase.AdminOpsUseCase.
type AdminOpsUseCase interface {
	ListBookings(ctx context.Context, filter entity.AdminBookingFilter) ([]entity.AdminBooking, int, error)
	GetBooking(ctx context.Context, bookingID string) (entity.AdminBooking, error)
	BookingMessages(ctx context.Context, bookingID string) ([]entity.BookingMessage, error)
	CancelBooking(ctx context.Context, bookingID string, refundAmount *int, note string) (entity.AdminBooking, error)
	RefundBooking(ctx context.Context, bookingID string, amount int, note string) (entity.AdminBooking, bool, error)
	ListReviews(ctx context.Context, hidden *bool, limit int) ([]entity.AdminReview, error)
	HideReview(ctx context.Context, reviewID, reason, adminID string) error
	RestoreReview(ctx context.Context, reviewID string) error
	CreateReport(ctx context.Context, reporterID, targetType, targetID, reason, details string) (entity.Report, error)
	ListReports(ctx context.Context, status string) ([]entity.Report, error)
	ResolveReport(ctx context.Context, reportID, status, note, adminID string) (entity.Report, error)
	CountPendingReports(ctx context.Context) (int, error)
	Metrics(ctx context.Context, days int) (entity.AdminMetrics, error)
}

// AdminOpsHandler serves the admin panel's tools over bookings (search, cancel, refund, chat),
// reviews (hide/restore), reports and the dashboard, plus POST /reports for the app. The
// /admin/* routes are only reachable through the gateway's ADMIN routes.
type AdminOpsHandler struct {
	useCase  AdminOpsUseCase
	location *time.Location
}

func NewAdminOpsHandler(useCase AdminOpsUseCase, location *time.Location) *AdminOpsHandler {
	return &AdminOpsHandler{useCase: useCase, location: location}
}

func (handler *AdminOpsHandler) RegisterRoutes(router chi.Router) {
	router.Get("/admin/bookings", handler.listBookings)
	router.Get("/admin/bookings/{id}", handler.getBooking)
	router.Get("/admin/bookings/{id}/messages", handler.bookingMessages)
	router.Post("/admin/bookings/{id}/cancel", handler.cancelBooking)
	router.Post("/admin/bookings/{id}/refund", handler.refundBooking)
	router.Get("/admin/reviews", handler.listReviews)
	router.Post("/admin/reviews/{id}/hide", handler.hideReview)
	router.Post("/admin/reviews/{id}/restore", handler.restoreReview)
	router.Post("/reports", handler.createReport)
	router.Get("/admin/reports", handler.listReports)
	router.Get("/admin/reports/count", handler.countReports)
	router.Post("/admin/reports/{id}/resolve", handler.resolveReport)
	router.Get("/admin/metrics", handler.metrics)
}

type adminBookingResponse struct {
	ID                 string  `json:"id"`
	ParkingSpotID      string  `json:"parking_spot_id"`
	UserID             string  `json:"user_id"`
	Start              string  `json:"start"`
	End                string  `json:"end"`
	Status             string  `json:"status"`
	Kind               string  `json:"kind"`
	Amount             int     `json:"amount"`
	OwnerAmount        *int    `json:"owner_amount,omitempty"`
	RefundedAmount     int     `json:"refunded_amount"`
	PaymentID          string  `json:"payment_id,omitempty"`
	PaidAt             *string `json:"paid_at,omitempty"`
	CancellationReason *string `json:"cancellation_reason,omitempty"`
	AdminNote          *string `json:"admin_note,omitempty"`
	// The owner's reason, when they cancelled a booking they had accepted.
	CancellationNote *string `json:"cancellation_note,omitempty"`
	// Set on an extension: the booking it continues.
	ParentBookingID    *string `json:"parent_booking_id,omitempty"`
	VerificationReview *string `json:"verification_review,omitempty"`
	VerifiedAt         *string `json:"verified_at,omitempty"`
	VehiclePlate       *string `json:"vehicle_plate,omitempty"`
	VehicleBrand       *string `json:"vehicle_brand,omitempty"`
	VehicleModel       *string `json:"vehicle_model,omitempty"`
	VehicleType        *string `json:"vehicle_type,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

func (handler *AdminOpsHandler) formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.In(handler.location).Format(time.RFC3339)
	return &formatted
}

func (handler *AdminOpsHandler) bookingResponse(booking entity.AdminBooking) adminBookingResponse {
	return adminBookingResponse{
		ID:                 booking.ID,
		ParkingSpotID:      booking.ParkingSpotID,
		UserID:             booking.UserID,
		Start:              booking.Start.In(handler.location).Format(time.RFC3339),
		End:                booking.End.In(handler.location).Format(time.RFC3339),
		Status:             booking.Status.String(),
		Kind:               booking.Kind.String(),
		Amount:             booking.Amount,
		OwnerAmount:        booking.OwnerAmount,
		RefundedAmount:     booking.RefundedAmount,
		PaymentID:          booking.PaymentID,
		PaidAt:             handler.formatTime(booking.PaidAt),
		CancellationReason: booking.CancellationReason,
		AdminNote:          booking.AdminNote,
		CancellationNote:   booking.CancellationNote,
		ParentBookingID:    booking.ParentBookingID,
		VerificationReview: booking.VerificationReview,
		VerifiedAt:         handler.formatTime(booking.VerifiedAt),
		VehiclePlate:       booking.VehiclePlate,
		VehicleBrand:       booking.VehicleBrand,
		VehicleModel:       booking.VehicleModel,
		VehicleType:        booking.VehicleType,
		CreatedAt:          booking.CreatedAt.In(handler.location).Format(time.RFC3339),
		UpdatedAt:          booking.UpdatedAt.In(handler.location).Format(time.RFC3339),
	}
}

func parseOptionalTime(raw string) (*time.Time, bool) {
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func (handler *AdminOpsHandler) listBookings(responseWriter http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	from, okFrom := parseOptionalTime(query.Get("from"))
	to, okTo := parseOptionalTime(query.Get("to"))
	if !okFrom || !okTo {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_DATE", "from/to must be RFC3339")
		return
	}
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))
	bookings, total, err := handler.useCase.ListBookings(request.Context(), entity.AdminBookingFilter{
		Query:         query.Get("q"),
		Status:        query.Get("status"),
		ParkingSpotID: query.Get("spot_id"),
		UserID:        query.Get("user_id"),
		From:          from,
		To:            to,
		Limit:         limit,
		Offset:        max(offset, 0),
	})
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	items := make([]adminBookingResponse, len(bookings))
	for i, booking := range bookings {
		items[i] = handler.bookingResponse(booking)
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{"items": items, "total": total})
}

func bookingIDParam(responseWriter http.ResponseWriter, request *http.Request) (string, bool) {
	bookingID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(bookingID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BOOKING_ID", "id must be a valid UUID")
		return "", false
	}
	return bookingID, true
}

func writeAdminError(responseWriter http.ResponseWriter, err error, notFoundCode string) {
	if errors.IsNotFound(err) {
		writeErrorCode(responseWriter, http.StatusNotFound, notFoundCode, err.Error())
		return
	}
	writeDomainError(responseWriter, err)
}

func (handler *AdminOpsHandler) getBooking(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	booking, err := handler.useCase.GetBooking(request.Context(), bookingID)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, handler.bookingResponse(booking))
}

func (handler *AdminOpsHandler) bookingMessages(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	messages, err := handler.useCase.BookingMessages(request.Context(), bookingID)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	items := make([]messageResponse, len(messages))
	for i, message := range messages {
		items[i] = messageResponseFromEntity(message, handler.location)
	}
	writeJSON(responseWriter, http.StatusOK, items)
}

type adminCancelRequest struct {
	// Centavos; omitted = everything the driver paid.
	RefundAmount *int   `json:"refund_amount"`
	Note         string `json:"note"`
}

func (handler *AdminOpsHandler) cancelBooking(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	var body adminCancelRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	booking, err := handler.useCase.CancelBooking(request.Context(), bookingID, body.RefundAmount, body.Note)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, handler.bookingResponse(booking))
}

type adminRefundRequest struct {
	// Centavos.
	Amount int    `json:"amount"`
	Note   string `json:"note"`
}

func (handler *AdminOpsHandler) refundBooking(responseWriter http.ResponseWriter, request *http.Request) {
	bookingID, ok := bookingIDParam(responseWriter, request)
	if !ok {
		return
	}
	var body adminRefundRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	booking, absorbed, err := handler.useCase.RefundBooking(request.Context(), bookingID, body.Amount, body.Note)
	if err != nil {
		writeAdminError(responseWriter, err, "BOOKING_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{
		"booking": handler.bookingResponse(booking),
		// The owner had already been paid for it: ParkLink absorbs this refund.
		"absorbed_by_parklink": absorbed,
	})
}

type adminReviewResponse struct {
	ID           string  `json:"id"`
	BookingID    string  `json:"booking_id"`
	ReviewerID   string  `json:"reviewer_id"`
	TargetType   string  `json:"target_type"`
	TargetID     string  `json:"target_id"`
	Rating       int     `json:"rating"`
	Comment      string  `json:"comment"`
	CreatedAt    string  `json:"created_at"`
	HiddenAt     *string `json:"hidden_at,omitempty"`
	HiddenReason *string `json:"hidden_reason,omitempty"`
}

func (handler *AdminOpsHandler) listReviews(responseWriter http.ResponseWriter, request *http.Request) {
	var hidden *bool
	switch request.URL.Query().Get("hidden") {
	case "true":
		value := true
		hidden = &value
	case "false":
		value := false
		hidden = &value
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	reviews, err := handler.useCase.ListReviews(request.Context(), hidden, limit)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	items := make([]adminReviewResponse, len(reviews))
	for i, review := range reviews {
		items[i] = adminReviewResponse{
			ID: review.ID, BookingID: review.BookingID, ReviewerID: review.ReviewerID,
			TargetType: review.TargetType.String(), TargetID: review.TargetID,
			Rating: review.Rating, Comment: review.Comment,
			CreatedAt:    review.CreatedAt.In(handler.location).Format(time.RFC3339),
			HiddenAt:     handler.formatTime(review.HiddenAt),
			HiddenReason: review.HiddenReason,
		}
	}
	writeJSON(responseWriter, http.StatusOK, items)
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

func (handler *AdminOpsHandler) hideReview(responseWriter http.ResponseWriter, request *http.Request) {
	reviewID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(reviewID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_REVIEW_ID", "id must be a valid UUID")
		return
	}
	var body reasonRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	if err := handler.useCase.HideReview(request.Context(), reviewID, body.Reason, request.Header.Get("X-User-Id")); err != nil {
		writeAdminError(responseWriter, err, "REVIEW_NOT_FOUND")
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *AdminOpsHandler) restoreReview(responseWriter http.ResponseWriter, request *http.Request) {
	reviewID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(reviewID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_REVIEW_ID", "id must be a valid UUID")
		return
	}
	if err := handler.useCase.RestoreReview(request.Context(), reviewID); err != nil {
		writeAdminError(responseWriter, err, "REVIEW_NOT_FOUND")
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

type reportResponse struct {
	ID         string  `json:"id"`
	ReporterID string  `json:"reporter_id"`
	TargetType string  `json:"target_type"`
	TargetID   string  `json:"target_id"`
	Reason     string  `json:"reason"`
	Details    string  `json:"details"`
	Status     string  `json:"status"`
	AdminNote  *string `json:"admin_note,omitempty"`
	ResolvedAt *string `json:"resolved_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

func (handler *AdminOpsHandler) reportResponse(report entity.Report) reportResponse {
	return reportResponse{
		ID: report.ID, ReporterID: report.ReporterID, TargetType: report.TargetType, TargetID: report.TargetID,
		Reason: report.Reason, Details: report.Details, Status: report.Status, AdminNote: report.AdminNote,
		ResolvedAt: handler.formatTime(report.ResolvedAt),
		CreatedAt:  report.CreatedAt.In(handler.location).Format(time.RFC3339),
	}
}

type createReportRequest struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Details    string `json:"details"`
}

func (handler *AdminOpsHandler) createReport(responseWriter http.ResponseWriter, request *http.Request) {
	reporterID := request.Header.Get("X-User-Id")
	if reporterID == "" {
		writeErrorCode(responseWriter, http.StatusBadRequest, "MISSING_USER_ID", "x-user-id header is required")
		return
	}
	var body createReportRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	report, err := handler.useCase.CreateReport(request.Context(), reporterID, body.TargetType, body.TargetID, body.Reason, body.Details)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusCreated, handler.reportResponse(report))
}

func (handler *AdminOpsHandler) listReports(responseWriter http.ResponseWriter, request *http.Request) {
	reports, err := handler.useCase.ListReports(request.Context(), request.URL.Query().Get("status"))
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	items := make([]reportResponse, len(reports))
	for i, report := range reports {
		items[i] = handler.reportResponse(report)
	}
	writeJSON(responseWriter, http.StatusOK, items)
}

func (handler *AdminOpsHandler) countReports(responseWriter http.ResponseWriter, request *http.Request) {
	count, err := handler.useCase.CountPendingReports(request.Context())
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]int{"count": count})
}

type resolveReportRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func (handler *AdminOpsHandler) resolveReport(responseWriter http.ResponseWriter, request *http.Request) {
	reportID := chi.URLParam(request, "id")
	if _, err := uuid.Parse(reportID); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_REPORT_ID", "id must be a valid UUID")
		return
	}
	var body resolveReportRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeErrorCode(responseWriter, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	report, err := handler.useCase.ResolveReport(request.Context(), reportID, body.Status, body.Note, request.Header.Get("X-User-Id"))
	if err != nil {
		writeAdminError(responseWriter, err, "REPORT_NOT_FOUND")
		return
	}
	writeJSON(responseWriter, http.StatusOK, handler.reportResponse(report))
}

type dailyBookingsResponse struct {
	Date   string `json:"date"`
	Paid   int    `json:"paid"`
	Amount int    `json:"amount"`
}

func (handler *AdminOpsHandler) metrics(responseWriter http.ResponseWriter, request *http.Request) {
	days, _ := strconv.Atoi(request.URL.Query().Get("days"))
	metrics, err := handler.useCase.Metrics(request.Context(), days)
	if err != nil {
		writeDomainError(responseWriter, err)
		return
	}
	daily := make([]dailyBookingsResponse, len(metrics.Daily))
	for i, day := range metrics.Daily {
		daily[i] = dailyBookingsResponse{Date: day.Date, Paid: day.Paid, Amount: day.Amount}
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{
		"days":          metrics.Days,
		"paid_bookings": metrics.PaidBookings,
		"gross_amount":  metrics.GrossAmount,
		"commission":    metrics.Commission,
		"refunded":      metrics.Refunded,
		"status_counts": metrics.StatusCounts,
		"daily":         daily,
	})
}
