CREATE TABLE IF NOT EXISTS booking_status_history (
  id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id  UUID          NOT NULL REFERENCES bookings(id),
  from_status booking_status,
  to_status   booking_status NOT NULL,
  reason      TEXT,
  created_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bsh_booking_id ON booking_status_history (booking_id);
