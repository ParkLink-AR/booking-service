package booking

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

// payoutResponse is the wire shape of a Payout row for ParkLink staff's admin panel.
type payoutResponse struct {
	ID               string `json:"id"`
	BookingID        string `json:"booking_id"`
	OwnerID          string `json:"owner_id"`
	PaymentID        string `json:"payment_id"`
	GrossAmount      int    `json:"gross_amount"`
	CommissionAmount int    `json:"commission_amount"`
	NetAmount        int    `json:"net_amount"`
	// Kind is BOOKING, or OVERSTAY for what ParkLink pays the owner for time past the booking's end.
	Kind          string  `json:"kind"`
	Status        string  `json:"status"`
	PaidAt        *string `json:"paid_at,omitempty"`
	PaidByAdminID *string `json:"paid_by_admin_id,omitempty"`
	ProofURL      *string `json:"proof_url,omitempty"`
	// SettlementID is set once a Settlement draft has swept this payout up -- see
	// entity.Payout.SettlementID. Nil while the payout is unattached.
	SettlementID *string `json:"settlement_id,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

func payoutResponseFromEntity(payout entity.Payout, location *time.Location) payoutResponse {
	response := payoutResponse{
		ID:               payout.ID,
		BookingID:        payout.BookingID,
		OwnerID:          payout.OwnerID,
		PaymentID:        payout.PaymentID,
		GrossAmount:      payout.GrossAmount,
		CommissionAmount: payout.CommissionAmount,
		NetAmount:        payout.NetAmount,
		Kind:             payout.Kind,
		Status:           payout.Status.String(),
		PaidByAdminID:    payout.PaidByAdminID,
		ProofURL:         payout.ProofURL,
		SettlementID:     payout.SettlementID,
		CreatedAt:        payout.CreatedAt.In(location).Format(time.RFC3339),
	}
	if payout.PaidAt != nil {
		formatted := payout.PaidAt.In(location).Format(time.RFC3339)
		response.PaidAt = &formatted
	}
	return response
}

// commissionResponse is the wire shape for GET/PUT /admin/settings/commission.
type commissionResponse struct {
	CommissionRateBP      int     `json:"commission_rate_bp"`
	CommissionRatePercent float64 `json:"commission_rate_percent"`
}

func commissionResponseFromEntity(settings entity.PlatformSettings) commissionResponse {
	return commissionResponse{
		CommissionRateBP:      settings.CommissionRateBP,
		CommissionRatePercent: float64(settings.CommissionRateBP) / 100,
	}
}

// setCommissionRequest is the body of PUT /admin/settings/commission.
type setCommissionRequest struct {
	CommissionRateBP int `json:"commission_rate_bp"`
}

// ownerBookingResponse is the wire shape for GET /admin/owners/{id}/bookings -- deliberately
// without VerificationCode/VerifiedAt (see bookingResponse): there's no in-person checkpoint
// for admin staff to protect the way there is from the spot's own owner, but this endpoint
// also has no reason to expose it, so it's left off entirely rather than reusing
// bookingResponse's viewer-matching trick.
type ownerBookingResponse struct {
	ID                 string  `json:"id"`
	ParkingSpotID      string  `json:"parking_spot_id"`
	UserID             string  `json:"user_id"`
	Start              string  `json:"start"`
	End                string  `json:"end"`
	Status             string  `json:"status"`
	Amount             int     `json:"amount"`
	PaymentID          string  `json:"payment_id,omitempty"`
	PaidAt             *string `json:"paid_at,omitempty"`
	CancellationReason *string `json:"cancellation_reason,omitempty"`
	Kind               string  `json:"kind"`
	UpdatedAt          string  `json:"updated_at"`
}

func ownerBookingResponseFromEntity(booking entity.Booking, location *time.Location) ownerBookingResponse {
	response := ownerBookingResponse{
		ID:            booking.ID,
		ParkingSpotID: booking.ParkingSpotID,
		UserID:        booking.UserID,
		Start:         booking.Start.In(location).Format(time.RFC3339),
		End:           booking.End.In(location).Format(time.RFC3339),
		Status:        booking.Status.String(),
		Amount:        booking.Amount,
		PaymentID:     booking.PaymentID,
		Kind:          booking.Kind.String(),
		UpdatedAt:     booking.UpdatedAt.In(location).Format(time.RFC3339),
	}
	if booking.PaidAt != nil {
		formatted := booking.PaidAt.In(location).Format(time.RFC3339)
		response.PaidAt = &formatted
	}
	response.CancellationReason = booking.CancellationReason
	return response
}
