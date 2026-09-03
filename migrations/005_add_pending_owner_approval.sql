ALTER TYPE booking_status ADD VALUE IF NOT EXISTS 'PENDING_OWNER_APPROVAL';

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS owner_response_deadline TIMESTAMPTZ;

-- A booking awaiting the owner's decision still occupies the slot, so it
-- has to keep blocking the same window for everyone else.
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_parking_spot_id_time_range_excl;
ALTER TABLE bookings ADD CONSTRAINT bookings_parking_spot_id_time_range_excl
  EXCLUDE USING GIST (parking_spot_id WITH =, time_range WITH &&)
  WHERE (status IN ('PENDING_PAYMENT', 'CONFIRMED', 'PENDING_OWNER_APPROVAL'));
