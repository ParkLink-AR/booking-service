CREATE TABLE IF NOT EXISTS bookings (
  id                       UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  parking_spot_id          UUID          NOT NULL,
  user_id                  UUID          NOT NULL,
  time_range               TSTZRANGE     NOT NULL,
  status                   booking_status NOT NULL,
  hold_expires_at          TIMESTAMPTZ,
  owner_response_deadline  TIMESTAMPTZ,
  amount                   INT           NOT NULL,
  payment_id               TEXT,
  created_at               TIMESTAMPTZ   NOT NULL DEFAULT now(),
  updated_at               TIMESTAMPTZ   NOT NULL DEFAULT now(),

  -- A booking that's PENDING_PAYMENT, CONFIRMED, or awaiting the owner's
  -- decision still occupies the slot, so all three keep blocking the same
  -- window for everyone else.
  EXCLUDE USING GIST (
    parking_spot_id WITH =,
    time_range WITH &&
  ) WHERE (status IN ('PENDING_PAYMENT', 'CONFIRMED', 'PENDING_OWNER_APPROVAL'))
);

CREATE INDEX IF NOT EXISTS idx_bookings_spot_range
  ON bookings USING GIST (parking_spot_id, time_range);
