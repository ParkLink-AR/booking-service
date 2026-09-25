package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// Payout represents the money ParkLink owes a parking spot's owner for a single completed,
// paid booking, net of platform commission. There is exactly one payout per booking (see the
// UNIQUE constraint on booking_id in migrations/014_create_payouts.sql) -- created once the
// booking has been COMPLETED and paid for at least 24h (see
// usecase.GeneratePendingPayoutsUseCase), and transitioned PENDING -> PAID by ParkLink staff
// once the bank transfer has actually gone out (see usecase.MarkPayoutPaidUseCase).
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
	// PaidAt/PaidByAdminID/ProofURL are all set together, once, by MarkPayoutPaidUseCase --
	// nil until the payout leaves PENDING.
	PaidAt        *time.Time
	PaidByAdminID *string
	// ProofURL points at the bank-transfer proof image ParkLink staff optionally attach
	// when marking the payout paid, uploaded to R2 (see internal/repository/storage). Nil
	// when no proof was attached.
	ProofURL  *string
	CreatedAt time.Time
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
