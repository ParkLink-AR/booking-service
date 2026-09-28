package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

// settlementPayoutResponse is the wire shape of a single payout inside a settlement's
// "payouts" list -- deliberately smaller than payoutResponse: the settlement detail view is
// about which bookings it covers, not each payout's own (now redundant) paid_at/proof_url,
// which mirror the settlement's own.
type settlementPayoutResponse struct {
	ID               string `json:"id"`
	BookingID        string `json:"booking_id"`
	GrossAmount      int    `json:"gross_amount"`
	CommissionAmount int    `json:"commission_amount"`
	NetAmount        int    `json:"net_amount"`
}

// settlementResponse is the wire shape of a Settlement, for both ParkLink staff's admin panel
// and the owner's own self-service "Finanzas" section. Payouts is only populated for detail
// views (GET /admin/settlements/{id}, GET /settlements/{id}) -- list endpoints omit it.
type settlementResponse struct {
	ID               string                     `json:"id"`
	OwnerID          string                     `json:"owner_id"`
	GrossAmount      int                        `json:"gross_amount"`
	CommissionAmount int                        `json:"commission_amount"`
	NetAmount        int                        `json:"net_amount"`
	Status           string                     `json:"status"`
	ProofURL         *string                    `json:"proof_url,omitempty"`
	PaidAt           *string                    `json:"paid_at,omitempty"`
	PaidByAdminID    *string                    `json:"paid_by_admin_id,omitempty"`
	CreatedByAdminID string                     `json:"created_by_admin_id"`
	CreatedAt        string                     `json:"created_at"`
	Payouts          []settlementPayoutResponse `json:"payouts,omitempty"`
}

// settlementResponseFromEntity builds a settlementResponse. Pass a nil payouts slice for list
// endpoints, or the settlement's covered payouts for a detail view.
func settlementResponseFromEntity(settlement entity.Settlement, payouts []entity.Payout, location *time.Location) settlementResponse {
	response := settlementResponse{
		ID:               settlement.ID,
		OwnerID:          settlement.OwnerID,
		GrossAmount:      settlement.GrossAmount,
		CommissionAmount: settlement.CommissionAmount,
		NetAmount:        settlement.NetAmount,
		Status:           settlement.Status.String(),
		ProofURL:         settlement.ProofURL,
		PaidByAdminID:    settlement.PaidByAdminID,
		CreatedByAdminID: settlement.CreatedByAdminID,
		CreatedAt:        settlement.CreatedAt.In(location).Format(time.RFC3339),
	}
	if settlement.PaidAt != nil {
		formatted := settlement.PaidAt.In(location).Format(time.RFC3339)
		response.PaidAt = &formatted
	}
	if payouts != nil {
		response.Payouts = make([]settlementPayoutResponse, len(payouts))
		for i, p := range payouts {
			response.Payouts[i] = settlementPayoutResponse{
				ID:               p.ID,
				BookingID:        p.BookingID,
				GrossAmount:      p.GrossAmount,
				CommissionAmount: p.CommissionAmount,
				NetAmount:        p.NetAmount,
			}
		}
	}
	return response
}

// ownerPendingBalanceResponse is the wire shape of an OwnerPendingBalance row for
// GET /admin/owners/pending-balances.
type ownerPendingBalanceResponse struct {
	OwnerID          string `json:"owner_id"`
	PendingNetAmount int    `json:"pending_net_amount"`
	PendingCount     int    `json:"pending_count"`
	OldestPendingAt  string `json:"oldest_pending_at"`
}

func ownerPendingBalanceResponseFromEntity(balance entity.OwnerPendingBalance, location *time.Location) ownerPendingBalanceResponse {
	return ownerPendingBalanceResponse{
		OwnerID:          balance.OwnerID,
		PendingNetAmount: balance.PendingNetAmount,
		PendingCount:     balance.PendingCount,
		OldestPendingAt:  balance.OldestPendingAt.In(location).Format(time.RFC3339),
	}
}
