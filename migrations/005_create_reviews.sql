CREATE TABLE IF NOT EXISTS reviews (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id   UUID        NOT NULL REFERENCES bookings(id),
  reviewer_id  UUID        NOT NULL,
  target_type  TEXT        NOT NULL CHECK (target_type IN ('spot', 'owner', 'driver')),
  target_id    UUID        NOT NULL,
  rating       SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
  comment      TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

  UNIQUE (booking_id, reviewer_id, target_type)
);

CREATE INDEX IF NOT EXISTS idx_reviews_target ON reviews (target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_reviews_booking ON reviews (booking_id);
