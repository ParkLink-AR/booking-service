-- A parking spot can rent N identical places against the same id (a "playa"): up to N
-- overlapping active bookings are allowed. The EXCLUDE USING GIST constraint from
-- migration 003 only allowed one, so it's dropped here. Overlap is now enforced in
-- booking-service's repository under a per-spot advisory lock, checking the concurrent
-- active-booking count against the spot's capacity.

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS capacity INT NOT NULL DEFAULT 1;

-- Drop the auto-named EXCLUDE constraint on bookings, if it's still there.
DO $$
DECLARE cname text;
BEGIN
  SELECT conname INTO cname FROM pg_constraint
    WHERE conrelid = 'bookings'::regclass AND contype = 'x';
  IF cname IS NOT NULL THEN
    EXECUTE format('ALTER TABLE bookings DROP CONSTRAINT %I', cname);
  END IF;
END$$;

-- idx_bookings_spot_range (GIST on parking_spot_id, time_range) stays: the overlap
-- recount query still filters on `time_range && ...`.
