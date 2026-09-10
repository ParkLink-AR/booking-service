-- A booking is either HOURLY (the original slot-based rental, validated against the
-- spot's published weekly schedule) or STAY (day/week/month rental, validated and priced
-- against parking-service's stay config instead -- see CreateBooking in
-- internal/service/booking/create_booking.go). Both kinds are rows in the same table and
-- share the same time_range column, so the existing overlap machinery (advisory lock +
-- concurrent-count check in booking.Repository.CreateBooking, and every GIST-indexed
-- range query) keeps a stay and an hourly booking from double-booking the same spot for
-- free, with no extra logic.
--
-- Defaulting existing rows to HOURLY keeps every booking made before this feature shipped
-- correctly classified.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'HOURLY'
  CHECK (kind IN ('HOURLY', 'STAY'));
