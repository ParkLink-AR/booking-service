package booking

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

const adminBookingColumns = `
	id, parking_spot_id, user_id,
	lower(time_range), upper(time_range),
	status, hold_expires_at, owner_response_deadline, amount, owner_amount, COALESCE(payment_id, ''), paid_at,
	cancellation_reason, kind, verification_code, verification_attempts, verified_at, created_at, updated_at,
	vehicle_plate, vehicle_brand, vehicle_model, vehicle_country, vehicle_type,
	refunded_amount, admin_note, verification_review, cancellation_note, parent_booking_id`

func scanAdminBooking(row rowScanner) (entity.AdminBooking, error) {
	var b entity.AdminBooking
	var statusLabel, kindLabel string
	if err := row.Scan(
		&b.ID, &b.ParkingSpotID, &b.UserID,
		&b.Start, &b.End,
		&statusLabel, &b.HoldExpiresAt, &b.OwnerResponseDeadline, &b.Amount, &b.OwnerAmount, &b.PaymentID, &b.PaidAt,
		&b.CancellationReason, &kindLabel, &b.VerificationCode, &b.VerificationAttempts, &b.VerifiedAt, &b.CreatedAt, &b.UpdatedAt,
		&b.VehiclePlate, &b.VehicleBrand, &b.VehicleModel, &b.VehicleCountry, &b.VehicleType,
		&b.RefundedAmount, &b.AdminNote, &b.VerificationReview, &b.CancellationNote, &b.ParentBookingID,
	); err != nil {
		return entity.AdminBooking{}, err
	}
	b.Status, _ = parseBookingStatus(statusLabel)
	b.Kind, _ = parseBookingKind(kindLabel)
	return b, nil
}

// ListAdminBookings returns the bookings matching filter, newest first, plus how many match
// in all (for paging) -- the admin panel's booking search.
func (repository *Repository) ListAdminBookings(ctx context.Context, filter entity.AdminBookingFilter) ([]entity.AdminBooking, int, error) {
	var conditions []string
	var args []any
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		args = append(args, strings.ToLower(query)+"%", "%"+strings.ToUpper(strings.ReplaceAll(query, " ", ""))+"%")
		conditions = append(conditions, fmt.Sprintf(
			"(id::text LIKE $%d OR REPLACE(UPPER(COALESCE(vehicle_plate, '')), ' ', '') LIKE $%d)", len(args)-1, len(args)))
	}
	if filter.Status != "" {
		add("status = $%d::booking_status", filter.Status)
	}
	if filter.ParkingSpotID != "" {
		add("parking_spot_id::text = $%d", filter.ParkingSpotID)
	}
	if filter.UserID != "" {
		add("user_id::text = $%d", filter.UserID)
	}
	if filter.From != nil {
		add("upper(time_range) > $%d", *filter.From)
	}
	if filter.To != nil {
		add("lower(time_range) < $%d", *filter.To)
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := repository.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+constants.BookingsTableName+` `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, errors.NewInternalError("booking.ListAdminBookings", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	pagedArgs := append(append([]any{}, args...), limit, filter.Offset)
	rows, err := repository.database.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s FROM %s %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, adminBookingColumns, constants.BookingsTableName, where, len(args)+1, len(args)+2), pagedArgs...)
	if err != nil {
		return nil, 0, errors.NewInternalError("booking.ListAdminBookings", err)
	}
	defer rows.Close()

	var bookings []entity.AdminBooking
	for rows.Next() {
		booking, err := scanAdminBooking(rows)
		if err != nil {
			return nil, 0, errors.NewInternalError("booking.ListAdminBookings", err)
		}
		bookings = append(bookings, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errors.NewInternalError("booking.ListAdminBookings", err)
	}
	return bookings, total, nil
}

// GetAdminBooking returns one booking with its admin-only fields.
func (repository *Repository) GetAdminBooking(ctx context.Context, bookingID string) (entity.AdminBooking, error) {
	booking, err := scanAdminBooking(repository.database.QueryRowContext(ctx,
		`SELECT `+adminBookingColumns+` FROM `+constants.BookingsTableName+` WHERE id = $1`, bookingID))
	if err != nil {
		if err == sql.ErrNoRows {
			return entity.AdminBooking{}, errors.NewNotFound("booking", bookingID)
		}
		return entity.AdminBooking{}, errors.NewInternalError("booking.GetAdminBooking", err)
	}
	return booking, nil
}

// AdminCancelBooking cancels a booking that hasn't finished on ParkLink's behalf, recording
// how much of it is being refunded and why. Only bookings still holding their slot can be
// cancelled; anything else is a conflict.
func (repository *Repository) AdminCancelBooking(ctx context.Context, bookingID string, refundedAmount int, note string) (entity.AdminBooking, error) {
	booking, err := scanAdminBooking(repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET status = 'CANCELLED', cancellation_reason = 'ADMIN_CANCELLED',
		    refunded_amount = $2, admin_note = $3, updated_at = now()
		WHERE id = $1 AND status IN ('PENDING_PAYMENT', 'PENDING_OWNER_APPROVAL', 'CONFIRMED', 'IN_PROGRESS')
		RETURNING `+adminBookingColumns, bookingID, refundedAmount, note))
	if err != nil {
		if err == sql.ErrNoRows {
			return entity.AdminBooking{}, errors.NewConflict("la reserva ya no se puede cancelar")
		}
		return entity.AdminBooking{}, errors.NewInternalError("booking.AdminCancelBooking", err)
	}
	return booking, nil
}

// RecordAdminRefund adds amount to what was given back for a booking (never past what was
// paid) and keeps the admin's note.
func (repository *Repository) RecordAdminRefund(ctx context.Context, bookingID string, amount int, note string) (entity.AdminBooking, error) {
	booking, err := scanAdminBooking(repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.BookingsTableName+`
		SET refunded_amount = refunded_amount + $2, admin_note = $3, updated_at = now()
		WHERE id = $1 AND refunded_amount + $2 <= amount
		RETURNING `+adminBookingColumns, bookingID, amount, note))
	if err != nil {
		if err == sql.ErrNoRows {
			return entity.AdminBooking{}, errors.NewInvalidInput("REFUND_TOO_LARGE", "el reembolso supera lo que se pagó")
		}
		return entity.AdminBooking{}, errors.NewInternalError("booking.RecordAdminRefund", err)
	}
	return booking, nil
}

// GetPayoutByBooking returns the owner's payout for a booking, if one was generated.
func (repository *Repository) GetPayoutByBooking(ctx context.Context, bookingID string) (*entity.Payout, error) {
	var payout entity.Payout
	var statusLabel string
	err := repository.database.QueryRowContext(ctx, `
		SELECT id, booking_id, owner_id, gross_amount, commission_amount, net_amount, status, settlement_id
		FROM `+constants.PayoutsTableName+` WHERE booking_id = $1
	`, bookingID).Scan(&payout.ID, &payout.BookingID, &payout.OwnerID, &payout.GrossAmount,
		&payout.CommissionAmount, &payout.NetAmount, &statusLabel, &payout.SettlementID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.NewInternalError("booking.GetPayoutByBooking", err)
	}
	payout.Status, _ = parsePayoutStatus(statusLabel)
	return &payout, nil
}

// AdjustPayout rewrites a payout's amounts after part of its booking was refunded, or
// deletes it when nothing is left for the owner. Only an unsettled payout (in no settlement
// draft) can change -- see usecase.AdminRefundBookingUseCase.
func (repository *Repository) AdjustPayout(ctx context.Context, payoutID string, gross, commission, net int) error {
	var result sql.Result
	var err error
	if net <= 0 {
		result, err = repository.database.ExecContext(ctx, `
			DELETE FROM `+constants.PayoutsTableName+` WHERE id = $1 AND settlement_id IS NULL AND status = 'PENDING'
		`, payoutID)
	} else {
		result, err = repository.database.ExecContext(ctx, `
			UPDATE `+constants.PayoutsTableName+`
			SET gross_amount = $2, commission_amount = $3, net_amount = $4
			WHERE id = $1 AND settlement_id IS NULL AND status = 'PENDING'
		`, payoutID, gross, commission, net)
	}
	if err != nil {
		return errors.NewInternalError("booking.AdjustPayout", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return errors.NewConflict("el pago al propietario ya está en una liquidación")
	}
	return nil
}

// ListAdminReviews returns the latest reviews for moderation, newest first. hidden: nil =
// all, true = only hidden ones, false = only visible ones.
func (repository *Repository) ListAdminReviews(ctx context.Context, hidden *bool, limit int) ([]entity.AdminReview, error) {
	where := ""
	if hidden != nil {
		if *hidden {
			where = "WHERE hidden_at IS NOT NULL"
		} else {
			where = "WHERE hidden_at IS NULL"
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, booking_id, reviewer_id, target_type, target_id, rating, COALESCE(comment, ''), created_at,
		       hidden_at, hidden_reason, hidden_by
		FROM `+constants.ReviewsTableName+` `+where+`
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListAdminReviews", err)
	}
	defer rows.Close()

	var reviews []entity.AdminReview
	for rows.Next() {
		var review entity.AdminReview
		var targetTypeLabel string
		if err := rows.Scan(
			&review.ID, &review.BookingID, &review.ReviewerID, &targetTypeLabel, &review.TargetID,
			&review.Rating, &review.Comment, &review.CreatedAt,
			&review.HiddenAt, &review.HiddenReason, &review.HiddenBy,
		); err != nil {
			return nil, errors.NewInternalError("booking.ListAdminReviews", err)
		}
		review.TargetType, _ = enum.ParseReviewTargetType(targetTypeLabel)
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListAdminReviews", err)
	}
	return reviews, nil
}

// SetReviewHidden hides a review (with the reason and who did it) or, with hide=false,
// shows it again.
func (repository *Repository) SetReviewHidden(ctx context.Context, reviewID string, hide bool, reason, adminID string) error {
	var result sql.Result
	var err error
	if hide {
		result, err = repository.database.ExecContext(ctx, `
			UPDATE `+constants.ReviewsTableName+` SET hidden_at = now(), hidden_reason = $2, hidden_by = $3 WHERE id = $1
		`, reviewID, reason, adminID)
	} else {
		result, err = repository.database.ExecContext(ctx, `
			UPDATE `+constants.ReviewsTableName+` SET hidden_at = NULL, hidden_reason = NULL, hidden_by = NULL WHERE id = $1
		`, reviewID)
	}
	if err != nil {
		return errors.NewInternalError("booking.SetReviewHidden", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return errors.NewNotFound("review", reviewID)
	}
	return nil
}

const reportColumns = `id, reporter_id, target_type, target_id, reason, details, status, admin_note, resolved_by, resolved_at, created_at`

func scanReport(row rowScanner) (entity.Report, error) {
	var report entity.Report
	err := row.Scan(&report.ID, &report.ReporterID, &report.TargetType, &report.TargetID, &report.Reason,
		&report.Details, &report.Status, &report.AdminNote, &report.ResolvedBy, &report.ResolvedAt, &report.CreatedAt)
	return report, err
}

// CreateReport stores a report a user filed from the app.
func (repository *Repository) CreateReport(ctx context.Context, report entity.Report) (entity.Report, error) {
	created, err := scanReport(repository.database.QueryRowContext(ctx, `
		INSERT INTO `+constants.ReportsTableName+` (reporter_id, target_type, target_id, reason, details)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+reportColumns, report.ReporterID, report.TargetType, report.TargetID, report.Reason, report.Details))
	if err != nil {
		return entity.Report{}, errors.NewInternalError("booking.CreateReport", err)
	}
	return created, nil
}

// ListReports returns reports, the oldest pending first when listing pending ones and the
// newest first otherwise. status "" lists every report.
func (repository *Repository) ListReports(ctx context.Context, status string) ([]entity.Report, error) {
	query := `SELECT ` + reportColumns + ` FROM ` + constants.ReportsTableName
	args := []any{}
	order := " ORDER BY created_at DESC"
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
		if status == "PENDING" {
			order = " ORDER BY created_at ASC"
		}
	}
	rows, err := repository.database.QueryContext(ctx, query+order+" LIMIT 500", args...)
	if err != nil {
		return nil, errors.NewInternalError("booking.ListReports", err)
	}
	defer rows.Close()

	var reports []entity.Report
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, errors.NewInternalError("booking.ListReports", err)
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("booking.ListReports", err)
	}
	return reports, nil
}

// ResolveReport closes a report as RESOLVED or DISMISSED, with the admin's note.
func (repository *Repository) ResolveReport(ctx context.Context, reportID, status, note, adminID string) (entity.Report, error) {
	report, err := scanReport(repository.database.QueryRowContext(ctx, `
		UPDATE `+constants.ReportsTableName+`
		SET status = $2, admin_note = $3, resolved_by = $4, resolved_at = now()
		WHERE id = $1
		RETURNING `+reportColumns, reportID, status, note, adminID))
	if err != nil {
		if err == sql.ErrNoRows {
			return entity.Report{}, errors.NewNotFound("report", reportID)
		}
		return entity.Report{}, errors.NewInternalError("booking.ResolveReport", err)
	}
	return report, nil
}

// CountPendingReports is the admin menu's count of reports waiting on staff.
func (repository *Repository) CountPendingReports(ctx context.Context) (int, error) {
	var count int
	if err := repository.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+constants.ReportsTableName+` WHERE status = 'PENDING'`,
	).Scan(&count); err != nil {
		return 0, errors.NewInternalError("booking.CountPendingReports", err)
	}
	return count, nil
}

// GetAdminMetrics sums up bookings and money for the last `days` days (by when the booking
// was made), in location's calendar days, plus how many bookings are in each status.
func (repository *Repository) GetAdminMetrics(ctx context.Context, days int, location *time.Location) (entity.AdminMetrics, error) {
	metrics := entity.AdminMetrics{Days: days, StatusCounts: map[string]int{}}
	since := time.Now().In(location).AddDate(0, 0, -days+1)
	since = time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, location)

	// A booking cancelled with no refund recorded by an admin was refunded in full
	// automatically (owner rejected it, driver cancelled, ...). ParkLink keeps its
	// commission only on what wasn't given back.
	if err := repository.database.QueryRowContext(ctx, `
		WITH paid AS (
			SELECT amount, COALESCE(owner_amount, amount) AS owner_amount,
			       CASE WHEN status = 'CANCELLED' AND refunded_amount = 0 THEN amount ELSE refunded_amount END AS refunded
			FROM `+constants.BookingsTableName+`
			WHERE paid_at IS NOT NULL AND created_at >= $1
		)
		SELECT COUNT(*), COALESCE(SUM(amount), 0),
		       COALESCE(SUM((amount - owner_amount)::bigint * (amount - refunded) / NULLIF(amount, 0)), 0),
		       COALESCE(SUM(refunded), 0)
		FROM paid
	`, since).Scan(&metrics.PaidBookings, &metrics.GrossAmount, &metrics.Commission, &metrics.Refunded); err != nil {
		return entity.AdminMetrics{}, errors.NewInternalError("booking.GetAdminMetrics", err)
	}

	statusRows, err := repository.database.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM `+constants.BookingsTableName+` GROUP BY status`)
	if err != nil {
		return entity.AdminMetrics{}, errors.NewInternalError("booking.GetAdminMetrics", err)
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var status string
		var count int
		if err := statusRows.Scan(&status, &count); err != nil {
			return entity.AdminMetrics{}, errors.NewInternalError("booking.GetAdminMetrics", err)
		}
		metrics.StatusCounts[status] = count
	}

	dailyRows, err := repository.database.QueryContext(ctx, `
		SELECT to_char(day, 'YYYY-MM-DD'), COUNT(b.id), COALESCE(SUM(b.amount), 0)
		FROM generate_series($1::date, $2::date, interval '1 day') AS day
		LEFT JOIN `+constants.BookingsTableName+` b
		       ON b.paid_at IS NOT NULL AND (b.created_at AT TIME ZONE $3)::date = day::date
		GROUP BY day ORDER BY day
	`, since.Format("2006-01-02"), time.Now().In(location).Format("2006-01-02"), location.String())
	if err != nil {
		return entity.AdminMetrics{}, errors.NewInternalError("booking.GetAdminMetrics", err)
	}
	defer dailyRows.Close()
	for dailyRows.Next() {
		var day entity.DailyBookings
		if err := dailyRows.Scan(&day.Date, &day.Paid, &day.Amount); err != nil {
			return entity.AdminMetrics{}, errors.NewInternalError("booking.GetAdminMetrics", err)
		}
		metrics.Daily = append(metrics.Daily, day)
	}
	return metrics, nil
}
