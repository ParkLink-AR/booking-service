-- "Media estadía": a third kind of booking, HALF_DAY -- 12 hours at a closed price, priced
-- like a stay and ended like an hourly booking (see enum.BookingKind.TracksExit). The kind
-- column's CHECK (created inline by 007_add_booking_kind.sql, so named bookings_kind_check)
-- is replaced to accept it; every existing row is HOURLY or STAY and still passes.
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_kind_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_kind_check CHECK (kind IN ('HOURLY', 'STAY', 'HALF_DAY'));
