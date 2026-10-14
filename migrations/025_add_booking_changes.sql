-- Changes to a booking after it was made:
-- * an extension is its own booking, chained to the one it continues (parent_booking_id);
-- * the owner can cancel a booking they had accepted (OWNER_CANCELLED, with a reason in
--   cancellation_note), and a driver who cancels an accepted booking in its last hour gets
--   part of it back (DRIVER_LATE_CANCELLED).
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS parent_booking_id UUID REFERENCES bookings(id);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS cancellation_note TEXT;
CREATE INDEX IF NOT EXISTS idx_bookings_parent_booking_id ON bookings (parent_booking_id) WHERE parent_booking_id IS NOT NULL;

ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_cancellation_reason_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_cancellation_reason_check
	CHECK (cancellation_reason IN (
		'OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED',
		'OWNER_ACCOUNT_CLOSED', 'DRIVER_ACCOUNT_CLOSED', 'ADMIN_CANCELLED',
		'OWNER_CANCELLED', 'DRIVER_LATE_CANCELLED'
	));
