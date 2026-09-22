-- Records the moment a booking's payment was actually confirmed by Mercado Pago
-- (PENDING_PAYMENT -> PENDING_OWNER_APPROVAL, see ConfirmBooking), independent of
-- updated_at, which every later transition (approve/reject/cancel/verify) also bumps.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;
