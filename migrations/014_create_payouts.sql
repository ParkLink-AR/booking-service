-- Tracks what ParkLink owes a parking spot's owner for a single completed, paid booking,
-- net of platform commission -- created once the booking has been COMPLETED and paid for at
-- least 24h (see usecase.GeneratePendingPayoutsUseCase), and marked PAID by ParkLink staff
-- once the bank transfer has actually gone out (see usecase.MarkPayoutPaidUseCase). One row
-- per booking (UNIQUE on booking_id) -- a booking can never get two payouts.
CREATE TABLE IF NOT EXISTS payouts (
	id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
	booking_id        UUID        NOT NULL UNIQUE REFERENCES bookings(id),
	owner_id          UUID        NOT NULL,
	-- The booking's Mercado Pago payment id (entity.Booking.PaymentID) at the moment this
	-- payout was generated -- used as the proof-image filename in R2 (see
	-- internal/repository/storage) so a transfer receipt is identifiable by payment
	-- reference without joining back to bookings.
	payment_id        TEXT        NOT NULL,
	gross_amount      INT         NOT NULL,
	commission_amount INT         NOT NULL,
	net_amount        INT         NOT NULL,
	status            TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID')),
	paid_at           TIMESTAMPTZ,
	paid_by_admin_id  UUID,
	proof_url         TEXT,
	created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payouts_status ON payouts (status);
CREATE INDEX IF NOT EXISTS idx_payouts_owner ON payouts (owner_id);
