package booking

import (
	"context"
	"strings"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/admin_ops_repository_mock.go -package=mocks -source=./admin_ops_usecase.go

// maxAdminNoteLength keeps an admin's note (why they cancelled, refunded, hid a review...) to
// a short explanation.
const maxAdminNoteLength = 500

// AdminBookingStore is what the admin panel's booking tools need from the bookings table.
// Implemented by internal/repository/booking.
type AdminBookingStore interface {
	ListAdminBookings(ctx context.Context, filter entity.AdminBookingFilter) ([]entity.AdminBooking, int, error)
	GetAdminBooking(ctx context.Context, bookingID string) (entity.AdminBooking, error)
	AdminCancelBooking(ctx context.Context, bookingID string, refundedAmount int, note string) (entity.AdminBooking, error)
	RecordAdminRefund(ctx context.Context, bookingID string, amount int, note string) (entity.AdminBooking, error)
	// ReleaseDebts puts back to pending the balances a booking's payment had settled or reserved.
	ReleaseDebts(ctx context.Context, bookingID string) error
	GetPayoutByBooking(ctx context.Context, bookingID string) (*entity.Payout, error)
	AdjustPayout(ctx context.Context, payoutID string, gross, commission, net int) error
}

// PaymentAmountRefunder gives back a whole payment or part of it (centavos). Implemented by
// internal/repository/paymentclient.
type PaymentAmountRefunder interface {
	RefundPayment(ctx context.Context, paymentID string) error
	RefundPaymentAmount(ctx context.Context, paymentID string, amountCentavos int) error
}

// BookingMessageLister reads a booking's chat between its driver and the spot's owner.
// Implemented by internal/repository/booking.
type BookingMessageLister interface {
	ListMessagesByBooking(ctx context.Context, bookingID string) ([]entity.BookingMessage, error)
}

// AdminReviewStore lists reviews for moderation and hides or restores them.
// Implemented by internal/repository/booking.
type AdminReviewStore interface {
	ListAdminReviews(ctx context.Context, hidden *bool, limit int) ([]entity.AdminReview, error)
	SetReviewHidden(ctx context.Context, reviewID string, hide bool, reason, adminID string) error
}

// ReportStore keeps what users report from the app. Implemented by internal/repository/booking.
type ReportStore interface {
	CreateReport(ctx context.Context, report entity.Report) (entity.Report, error)
	ListReports(ctx context.Context, status string) ([]entity.Report, error)
	ResolveReport(ctx context.Context, reportID, status, note, adminID string) (entity.Report, error)
	CountPendingReports(ctx context.Context) (int, error)
}

// MetricsReader sums up bookings and money for the admin dashboard.
// Implemented by internal/repository/booking.
type MetricsReader interface {
	GetAdminMetrics(ctx context.Context, days int, location *time.Location) (entity.AdminMetrics, error)
}

// AdminOpsUseCase holds the admin panel's tools over bookings, reviews, reports and the
// dashboard: searching bookings, cancelling or refunding one, reading its chat, hiding a
// review, handling reports.
type AdminOpsUseCase struct {
	bookings AdminBookingStore
	refunder PaymentAmountRefunder
	messages BookingMessageLister
	reviews  AdminReviewStore
	reports  ReportStore
	metrics  MetricsReader
	spots    SpotDetailsLookup
	events   EventPublisher
	location *time.Location
}

func NewAdminOpsUseCase(
	bookings AdminBookingStore,
	refunder PaymentAmountRefunder,
	messages BookingMessageLister,
	reviews AdminReviewStore,
	reports ReportStore,
	metrics MetricsReader,
	spots SpotDetailsLookup,
	events EventPublisher,
	location *time.Location,
) *AdminOpsUseCase {
	return &AdminOpsUseCase{
		bookings: bookings, refunder: refunder, messages: messages, reviews: reviews,
		reports: reports, metrics: metrics, spots: spots, events: events, location: location,
	}
}

func (uc *AdminOpsUseCase) ListBookings(ctx context.Context, filter entity.AdminBookingFilter) ([]entity.AdminBooking, int, error) {
	return uc.bookings.ListAdminBookings(ctx, filter)
}

func (uc *AdminOpsUseCase) GetBooking(ctx context.Context, bookingID string) (entity.AdminBooking, error) {
	return uc.bookings.GetAdminBooking(ctx, bookingID)
}

// BookingMessages is the booking's chat between driver and owner, read-only for staff
// sorting out a dispute.
func (uc *AdminOpsUseCase) BookingMessages(ctx context.Context, bookingID string) ([]entity.BookingMessage, error) {
	if _, err := uc.bookings.GetAdminBooking(ctx, bookingID); err != nil {
		return nil, err
	}
	return uc.messages.ListMessagesByBooking(ctx, bookingID)
}

func cleanNote(note string) (string, error) {
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return "", errors.NewInvalidInput("NOTE_REQUIRED", "contá brevemente el motivo")
	}
	if len(trimmed) > maxAdminNoteLength {
		return "", errors.NewInvalidInput("NOTE_TOO_LONG", "el motivo es demasiado largo")
	}
	return trimmed, nil
}

// CancelBooking calls off a booking that hasn't finished, on ParkLink's behalf, giving back
// refundAmount centavos of what the driver paid (nil = all of it; 0 = nothing). Both sides
// are told. The owner had no payout for it yet (payouts only exist for finished bookings).
func (uc *AdminOpsUseCase) CancelBooking(ctx context.Context, bookingID string, refundAmount *int, note string) (entity.AdminBooking, error) {
	cleaned, err := cleanNote(note)
	if err != nil {
		return entity.AdminBooking{}, err
	}
	booking, err := uc.bookings.GetAdminBooking(ctx, bookingID)
	if err != nil {
		return entity.AdminBooking{}, err
	}
	// What the driver paid includes the pending balances the booking took on: giving back all of
	// it returns those too (they owe them again), anything less only touches the booking's share.
	paid := booking.Amount + booking.DebtAmount
	refund := 0
	if booking.PaidAt != nil {
		refund = paid
		if refundAmount != nil {
			refund = *refundAmount
		}
	}
	if refund < 0 || refund > paid {
		return entity.AdminBooking{}, errors.NewInvalidInput("INVALID_REFUND", "el reembolso tiene que estar entre 0 y lo que se pagó")
	}
	if refund > 0 && booking.PaidAt == nil {
		return entity.AdminBooking{}, errors.NewInvalidInput("NOT_PAID", "la reserva no se pagó: no hay nada para devolver")
	}

	// The booking's own record never holds more than its own price as refunded.
	cancelled, err := uc.bookings.AdminCancelBooking(ctx, bookingID, min(refund, booking.Amount), cleaned)
	if err != nil {
		return entity.AdminBooking{}, err
	}
	if booking.DebtAmount > 0 && (booking.PaidAt == nil || refund >= paid) {
		if err := uc.bookings.ReleaseDebts(ctx, bookingID); err != nil {
			return cancelled, err
		}
	}
	if refund > 0 {
		// The booking stays cancelled even if Mercado Pago fails: payment-service keeps the
		// refund on record and the "Reembolsos pendientes" screen follows it up.
		if err := uc.refund(ctx, cancelled.PaymentID, refund, paid); err != nil {
			return cancelled, err
		}
	}
	uc.announce(ctx, cancelled.Booking, AdminActionCancelled, refund)
	return cancelled, nil
}

// RefundBooking gives back amount centavos of a finished booking (a complaint, a dispute).
// The owner's payout follows: not generated yet, it'll be computed on what's left; pending
// and in no settlement, it's reduced (or dropped) right away; in a settlement being
// prepared, the refund waits until that settlement is cancelled; already paid to the owner,
// ParkLink absorbs it -- payoutAbsorbed says so.
func (uc *AdminOpsUseCase) RefundBooking(ctx context.Context, bookingID string, amount int, note string) (entity.AdminBooking, bool, error) {
	cleaned, err := cleanNote(note)
	if err != nil {
		return entity.AdminBooking{}, false, err
	}
	booking, err := uc.bookings.GetAdminBooking(ctx, bookingID)
	if err != nil {
		return entity.AdminBooking{}, false, err
	}
	if booking.Status != enum.BookingStatusCompleted && booking.Status != enum.BookingStatusInProgress {
		return entity.AdminBooking{}, false, errors.NewInvalidInput("NOT_FINISHED",
			"solo se reembolsa así una reserva en curso o terminada; para las demás, cancelala")
	}
	if booking.PaidAt == nil {
		return entity.AdminBooking{}, false, errors.NewInvalidInput("NOT_PAID", "la reserva no se pagó: no hay nada para devolver")
	}
	if amount <= 0 || booking.RefundedAmount+amount > booking.Amount {
		return entity.AdminBooking{}, false, errors.NewInvalidInput("INVALID_REFUND", "el reembolso supera lo que queda por devolver")
	}

	payout, err := uc.bookings.GetPayoutByBooking(ctx, bookingID)
	if err != nil {
		return entity.AdminBooking{}, false, err
	}
	absorbed := false
	if payout != nil {
		switch {
		case payout.Status == enum.PayoutStatusPaid:
			absorbed = true
		case payout.SettlementID != nil:
			return entity.AdminBooking{}, false, errors.NewConflict(
				"el pago al propietario está en una liquidación en preparación: cancelá esa liquidación y volvé a intentar")
		}
	}

	// Never the whole payment while it includes pending balances: only the booking's own share can
	// be given back this way.
	if err := uc.refund(ctx, booking.PaymentID, amount, booking.Amount+booking.DebtAmount); err != nil {
		return entity.AdminBooking{}, false, err
	}
	refunded, err := uc.bookings.RecordAdminRefund(ctx, bookingID, amount, cleaned)
	if err != nil {
		return entity.AdminBooking{}, false, err
	}
	if payout != nil && !absorbed {
		// The payout recomputed from the booking's full amount minus everything refunded so
		// far. Its commission share never changes, so the current payout gives it back even
		// for a booking that predates owner_amount.
		commission := 0
		if booking.OwnerAmount != nil {
			commission = booking.Amount - *booking.OwnerAmount
		} else if payout.GrossAmount > 0 {
			commission = payout.CommissionAmount * booking.Amount / payout.GrossAmount
		}
		gross, adjustedCommission, net := refundedPayoutAmounts(booking.Amount, commission, refunded.RefundedAmount)
		if err := uc.bookings.AdjustPayout(ctx, payout.ID, gross, adjustedCommission, net); err != nil {
			return refunded, false, err
		}
	}
	uc.announce(ctx, refunded.Booking, AdminActionRefunded, amount)
	return refunded, absorbed, nil
}

func (uc *AdminOpsUseCase) refund(ctx context.Context, paymentID string, amount, total int) error {
	if amount >= total {
		return uc.refunder.RefundPayment(ctx, paymentID)
	}
	return uc.refunder.RefundPaymentAmount(ctx, paymentID, amount)
}

// announce tells both sides what ParkLink did -- best effort: the spot is only needed for
// the notice, so a failed lookup never undoes the action.
func (uc *AdminOpsUseCase) announce(ctx context.Context, booking entity.Booking, action string, refunded int) {
	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return
	}
	emitBookingEvent(ctx, uc.events, RoutingKeyBookingAdminAction, newBookingAdminActionEvent(booking, spot, action, refunded))
}

func (uc *AdminOpsUseCase) ListReviews(ctx context.Context, hidden *bool, limit int) ([]entity.AdminReview, error) {
	return uc.reviews.ListAdminReviews(ctx, hidden, limit)
}

// HideReview takes a review out of every public listing and average (offensive, fake...).
func (uc *AdminOpsUseCase) HideReview(ctx context.Context, reviewID, reason, adminID string) error {
	cleaned, err := cleanNote(reason)
	if err != nil {
		return err
	}
	return uc.reviews.SetReviewHidden(ctx, reviewID, true, cleaned, adminID)
}

func (uc *AdminOpsUseCase) RestoreReview(ctx context.Context, reviewID string) error {
	return uc.reviews.SetReviewHidden(ctx, reviewID, false, "", "")
}

// Report targets and statuses.
var (
	reportTargetTypes   = map[string]bool{"spot": true, "user": true, "review": true, "booking": true}
	reportFinalStatuses = map[string]bool{"RESOLVED": true, "DISMISSED": true}
)

const (
	// How early before a booking starts either side can already report a problem with it
	// (the spot is taken, nobody answers the gate).
	bookingReportLead = 30 * time.Minute

	maxReportReasonLength  = 100
	maxReportDetailsLength = 1000
)

// CreateReport files a user's report about a spot, a person, a review, or something that
// went wrong with a booking of theirs (the driver never came or stayed past the end; the
// spot was taken).
func (uc *AdminOpsUseCase) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason, details string) (entity.Report, error) {
	if !reportTargetTypes[targetType] {
		return entity.Report{}, errors.NewInvalidInput("INVALID_TARGET", "target_type must be spot, user, review or booking")
	}
	if strings.TrimSpace(targetID) == "" {
		return entity.Report{}, errors.NewInvalidInput("INVALID_TARGET", "target_id is required")
	}
	if targetType == "user" && targetID == reporterID {
		return entity.Report{}, errors.NewInvalidInput("INVALID_TARGET", "no podés denunciarte a vos mismo")
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" || len(trimmedReason) > maxReportReasonLength {
		return entity.Report{}, errors.NewInvalidInput("INVALID_REASON", "elegí un motivo")
	}
	trimmedDetails := strings.TrimSpace(details)
	if len(trimmedDetails) > maxReportDetailsLength {
		return entity.Report{}, errors.NewInvalidInput("DETAILS_TOO_LONG", "el detalle es demasiado largo")
	}
	if targetType == "booking" {
		if err := uc.checkBookingReport(ctx, reporterID, targetID); err != nil {
			return entity.Report{}, err
		}
	}
	return uc.reports.CreateReport(ctx, entity.Report{
		ReporterID: reporterID, TargetType: targetType, TargetID: targetID, Reason: trimmedReason, Details: trimmedDetails,
	})
}

// checkBookingReport: a problem with a booking can only be reported by its driver or its
// spot's owner, on a booking that was accepted, from shortly before it starts.
func (uc *AdminOpsUseCase) checkBookingReport(ctx context.Context, reporterID, bookingID string) error {
	booking, err := uc.bookings.GetAdminBooking(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.UserID != reporterID {
		spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
		if err != nil {
			return err
		}
		if spot.OwnerID != reporterID {
			return errors.NewForbidden("no sos parte de esta reserva")
		}
	}
	switch booking.Status {
	case enum.BookingStatusConfirmed, enum.BookingStatusInProgress, enum.BookingStatusCompleted:
	default:
		return errors.NewInvalidInput("BOOKING_NOT_REPORTABLE", "solo se puede reportar un problema con una reserva confirmada")
	}
	if time.Now().Before(booking.Start.Add(-bookingReportLead)) {
		return errors.NewInvalidInput("BOOKING_NOT_STARTED", "todavía falta para que empiece la reserva")
	}
	return nil
}

func (uc *AdminOpsUseCase) ListReports(ctx context.Context, status string) ([]entity.Report, error) {
	if status != "" && status != "PENDING" && !reportFinalStatuses[status] {
		return nil, errors.NewInvalidInput("INVALID_STATUS", "status must be PENDING, RESOLVED or DISMISSED")
	}
	return uc.reports.ListReports(ctx, status)
}

// ResolveReport closes a report: RESOLVED (something was done) or DISMISSED (nothing to do).
func (uc *AdminOpsUseCase) ResolveReport(ctx context.Context, reportID, status, note, adminID string) (entity.Report, error) {
	if !reportFinalStatuses[status] {
		return entity.Report{}, errors.NewInvalidInput("INVALID_STATUS", "status must be RESOLVED or DISMISSED")
	}
	trimmed := strings.TrimSpace(note)
	if len(trimmed) > maxAdminNoteLength {
		return entity.Report{}, errors.NewInvalidInput("NOTE_TOO_LONG", "la nota es demasiado larga")
	}
	return uc.reports.ResolveReport(ctx, reportID, status, trimmed, adminID)
}

func (uc *AdminOpsUseCase) CountPendingReports(ctx context.Context) (int, error) {
	return uc.reports.CountPendingReports(ctx)
}

// Metrics is the dashboard's summary of the last `days` days (7 to 90).
func (uc *AdminOpsUseCase) Metrics(ctx context.Context, days int) (entity.AdminMetrics, error) {
	if days < 7 || days > 90 {
		days = 30
	}
	return uc.metrics.GetAdminMetrics(ctx, days, uc.location)
}
