-- Singleton row for ParkLink-wide settings the admin panel can change without a deploy --
-- currently just the platform commission rate (basis points; 500 = 5.00%), which used to be
-- effectively hardcoded. The `id BOOLEAN ... CHECK (id)` trick guarantees there can ever be
-- exactly one row -- a second INSERT collides on the primary key.
CREATE TABLE IF NOT EXISTS platform_settings (
	id                  BOOLEAN     PRIMARY KEY DEFAULT true CHECK (id),
	commission_rate_bp  INT         NOT NULL DEFAULT 500,
	updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by_admin_id UUID
);

INSERT INTO platform_settings (id, commission_rate_bp)
VALUES (true, 500)
ON CONFLICT (id) DO NOTHING;
