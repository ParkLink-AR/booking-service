package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// Payout represents the money ParkLink owes a parking spot's owner for a single completed,
// paid booking, net of platform commission. There is exactly one payout per booking (see the
// UNIQUE constraint on booking_id in migrations/014_create_payouts.sql) -- created once the
// booking has been COMPLETED and paid for at least 24h (see
// usecase.GeneratePendingPayoutsUseCase). Payouts are never paid out individually anymore: an
// admin drafts a Settlement that sweeps up every payout currently unattached for that owner
// (see usecase.DraftSettlementUseCase), then confirms it once the bank transfer has actually
// gone out (see usecase.ConfirmSettlementUseCase) -- confirmation is what flips Status to PAID
// and stamps PaidAt/PaidByAdminID/ProofURL here, mirroring the values recorded on the
// Settlement itself.
type Payout struct {
	ID        string
	BookingID string
	OwnerID   string
	// PaymentID is the booking's Mercado Pago payment id at the moment this payout was
	// generated -- used as the proof-image filename in R2 (see internal/repository/storage).
	PaymentID string
	// GrossAmount is the booking's full paid amount, in centavos (same convention as
	// entity.Booking.Amount), before commission is deducted.
	GrossAmount int
	// CommissionAmount is ParkLink's cut, computed at the platform commission rate in
	// effect when the payout was generated -- see entity.PlatformSettings.
	CommissionAmount int
	// NetAmount is what actually gets transferred to the owner: GrossAmount - CommissionAmount.
	NetAmount int
	Status    enum.PayoutStatus
	// SettlementID is set once a Settlement draft has swept this payout up (see
	// usecase.DraftSettlementUseCase) -- nil while the payout is unattached, which is exactly
	// the set GetOwnerPendingBalances/a future draft consider available. A payout can have a
	// non-nil SettlementID while Status is still PENDING (drafted but not yet confirmed);
	// Status only flips to PAID once usecase.ConfirmSettlementUseCase runs.
	SettlementID *string
	// PaidAt/PaidByAdminID/ProofURL are all set together, once, by ConfirmSettlementUseCase,
	// mirroring the owning Settlement's own values -- nil until the payout leaves PENDING.
	PaidAt        *time.Time
	PaidByAdminID *string
	// ProofURL points at the bank-transfer proof image ParkLink staff optionally attach
	// when confirming the settlement, uploaded to R2 (see internal/repository/storage). Nil
	// when no proof was attached.
	ProofURL  *string
	CreatedAt time.Time
}

// OwnerPendingBalance is one row per owner with at least one unattached (not yet part of any
// Settlement draft) payout -- powers GET /admin/owners/pending-balances, the admin-panel
// screen an admin uses to decide who to settle next. See
// internal/repository/booking/get_owner_pending_balances.go.
type OwnerPendingBalance struct {
	OwnerID          string
	PendingNetAmount int
	PendingCount     int
	OldestPendingAt  time.Time
}

// PlatformSettings holds ParkLink-wide configuration the admin panel can change without a
// deploy. Currently just the platform commission rate; a singleton row (see
// migrations/015_create_platform_settings.sql) so there's exactly one value in effect at
// any time, never a per-owner or per-spot override.
type PlatformSettings struct {
	// CommissionRateBP is the platform's cut in basis points (500 = 5.00%). Read by
	// GeneratePendingPayoutsUseCase when computing a new payout's commission, and by the
	// admin panel's settings screen.
	CommissionRateBP int
	UpdatedAt        time.Time
	UpdatedByAdminID *string
}
