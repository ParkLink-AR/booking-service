-- Staying past a booking's end beyond the grace period is charged (see entity.ComputeOverstay).
-- Closing the booking's exit turns that into a debt the driver carries until their next
-- reservation, whose payment simply includes it:
--   PENDING   nobody has asked for it yet
--   RESERVED  attached to a booking still waiting for its payment (charged_booking_id); goes back
--             to PENDING if that booking's hold runs out
--   PAID      that booking's payment was approved; goes back to PENDING if the payment is refunded
--             in full (a cancellation, a rejection...), stays PAID if only the booking's own share
--             was given back
-- One debt per booking (UNIQUE source_booking_id), so closing an exit twice can never charge twice.
CREATE TABLE IF NOT EXISTS booking_debts (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID        NOT NULL,
    source_booking_id  UUID        NOT NULL UNIQUE REFERENCES bookings(id),
    charged_booking_id UUID        REFERENCES bookings(id),
    minutes            INT         NOT NULL,
    amount             INT         NOT NULL CHECK (amount > 0),
    status             TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'RESERVED', 'PAID')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at            TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_booking_debts_user ON booking_debts (user_id, status);
CREATE INDEX IF NOT EXISTS idx_booking_debts_charged ON booking_debts (charged_booking_id);

-- What a booking charges on top of its own price: the debts it took on (booking_debts.charged_booking_id).
-- amount stays the booking's own price, so payouts and refund policies keep working on it.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS debt_amount INT NOT NULL DEFAULT 0;

-- ParkLink pays the owner for the overstay out of its own pocket, as a payout of its own next to
-- the booking's: a booking can now have one of each kind.
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'BOOKING' CHECK (kind IN ('BOOKING', 'OVERSTAY'));
ALTER TABLE payouts DROP CONSTRAINT IF EXISTS payouts_booking_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_payouts_booking_kind ON payouts (booking_id, kind);
