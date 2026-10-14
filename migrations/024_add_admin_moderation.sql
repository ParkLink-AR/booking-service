-- Admin tools (see usecase/booking/admin_*):
-- * a booking ParkLink staff cancelled or refunded by hand: how much of it went back to the
--   driver, and why (shown in the panel);
-- * reviews staff hid (offensive, fake) -- kept, but left out of every public listing and
--   average;
-- * reports users file from the app about a spot, a person or a review.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS refunded_amount INTEGER NOT NULL DEFAULT 0;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS admin_note TEXT;

ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_cancellation_reason_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_cancellation_reason_check
	CHECK (cancellation_reason IN (
		'OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED',
		'OWNER_ACCOUNT_CLOSED', 'DRIVER_ACCOUNT_CLOSED', 'ADMIN_CANCELLED'
	));

ALTER TABLE reviews ADD COLUMN IF NOT EXISTS hidden_at TIMESTAMPTZ;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS hidden_reason TEXT;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS hidden_by TEXT;

CREATE TABLE IF NOT EXISTS reports (
	id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	reporter_id  UUID NOT NULL,
	target_type  TEXT NOT NULL CHECK (target_type IN ('spot', 'user', 'review')),
	target_id    TEXT NOT NULL,
	reason       TEXT NOT NULL,
	details      TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'RESOLVED', 'DISMISSED')),
	admin_note   TEXT,
	resolved_by  TEXT,
	resolved_at  TIMESTAMPTZ,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_reports_pending ON reports (created_at) WHERE status = 'PENDING';
