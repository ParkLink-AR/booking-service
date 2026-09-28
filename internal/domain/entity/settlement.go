package entity

import (
	"time"

	"github.com/example/bookings-service/internal/domain/enum"
)

// Settlement is a two-phase payout event to a parking-spot owner, covering every Payout that
// was unattached for them at the moment an admin drafted it -- see usecase.DraftSettlementUseCase
// and usecase.ConfirmSettlementUseCase. Replaces paying out each Payout individually: an owner
// with 10 pending bookings gets ONE settlement (one bank transfer, one proof image) instead of
// 10.
//
// The two phases exist to close a race an admin could otherwise hit: if drafting the total AND
// confirming the transfer happened in one atomic step, a new payout becoming PENDING while the
// admin is away from the keyboard (between "I see I owe $X" and "I actually wired it") would
// get silently swept into a transfer the admin never intended to include. Splitting it in two
// draws a hard line between "I've decided what I'm about to pay" (DraftSettlementUseCase --
// Status PENDING, every currently-unattached payout for the owner gets SettlementID set to
// this row, but their own Status stays PENDING) and "I've confirmed the transfer went out"
// (ConfirmSettlementUseCase -- Status PAID, PaidAt/PaidByAdminID/ProofURL stamped, and this is
// the point settlement.paid is published). A drafted-but-unconfirmed settlement can be undone
// (see usecase.CancelSettlementUseCase), releasing its payouts back to unattached.
type Settlement struct {
	ID               string
	OwnerID          string
	GrossAmount      int
	CommissionAmount int
	NetAmount        int
	Status           enum.SettlementStatus
	// ProofURL points at the bank-transfer proof image an admin optionally attaches when
	// confirming the settlement, uploaded to R2 (see internal/repository/storage). Nil until
	// confirmed, and even then only if a proof was actually attached.
	ProofURL *string
	// PaidAt/PaidByAdminID are set together by ConfirmSettlementUseCase -- nil while Status
	// is still PENDING.
	PaidAt        *time.Time
	PaidByAdminID *string
	// CreatedByAdminID is the admin who drafted this settlement (DraftSettlementUseCase) --
	// may differ from PaidByAdminID, since a different admin can confirm it.
	CreatedByAdminID string
	CreatedAt        time.Time
}
